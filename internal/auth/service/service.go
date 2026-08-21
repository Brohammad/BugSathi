package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/Brohammad/BugSathi/internal/auth/port"
	"github.com/google/uuid"
)

type Service struct {
	users      port.UserRepository
	refresh    port.RefreshTokenRepository
	hasher     port.PasswordHasher
	tokens     port.TokenManager
	refreshTTL time.Duration
	reuseGrace time.Duration
	now        func() time.Time
}

func New(
	users port.UserRepository,
	refresh port.RefreshTokenRepository,
	hasher port.PasswordHasher,
	tokens port.TokenManager,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		users:      users,
		refresh:    refresh,
		hasher:     hasher,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		reuseGrace: 10 * time.Second,
		now:        time.Now,
	}
}

// WithReuseGrace overrides the refresh-theft detection grace window (tests).
func (s *Service) WithReuseGrace(d time.Duration) *Service {
	s.reuseGrace = d
	return s
}

type TokenPair struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	TokenType        string    `json:"token_type"`
}

type UserDTO struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toDTO(u domain.User) UserDTO {
	return UserDTO{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}

func (s *Service) Register(ctx context.Context, email, password, name string) (UserDTO, TokenPair, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return UserDTO{}, TokenPair{}, domain.ErrEmailTaken
	} else if !errors.Is(err, domain.ErrNotFound) {
		return UserDTO{}, TokenPair{}, err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	now := s.now()
	user, err := s.users.Create(ctx, domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	return toDTO(user), pair, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (UserDTO, TokenPair, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return UserDTO{}, TokenPair{}, domain.ErrInvalidCredentials
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return UserDTO{}, TokenPair{}, domain.ErrInvalidCredentials
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		return UserDTO{}, TokenPair{}, domain.ErrInvalidCredentials
	}
	pair, err := s.issuePair(ctx, user)
	if err != nil {
		return UserDTO{}, TokenPair{}, err
	}
	return toDTO(user), pair, nil
}

func (s *Service) Refresh(ctx context.Context, rawRefresh string) (TokenPair, error) {
	if rawRefresh == "" {
		return TokenPair{}, domain.ErrUnauthorized
	}
	rawNew, err := newOpaqueToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := s.now()
	exp := now.Add(s.refreshTTL)
	hash := hashToken(rawRefresh)
	consumed, err := s.refresh.Rotate(ctx, hash, now, domain.RefreshToken{
		ID:        uuid.New(),
		TokenHash: hashToken(rawNew),
		ExpiresAt: exp,
		CreatedAt: now,
	})
	if err != nil {
		s.revokeFamilyOnReuse(ctx, hash, now)
		return TokenPair{}, err
	}
	user, err := s.users.FindByID(ctx, consumed.UserID)
	if err != nil {
		return TokenPair{}, domain.ErrUnauthorized
	}
	access, accessExp, err := s.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     rawNew,
		RefreshExpiresAt: exp,
		TokenType:        "Bearer",
	}, nil
}

// revokeFamilyOnReuse detects refresh-token reuse (theft) and revokes every
// active refresh token for that user. A short grace window avoids wiping a
// legitimate concurrent refresh that just won the rotation race.
func (s *Service) revokeFamilyOnReuse(ctx context.Context, hash string, now time.Time) {
	rec, err := s.refresh.FindByHash(ctx, hash)
	if err != nil || rec.RevokedAt == nil {
		return
	}
	const reuseGrace = 10 * time.Second
	if now.Sub(*rec.RevokedAt) <= s.reuseGrace {
		return
	}
	_ = s.refresh.RevokeAllForUser(ctx, rec.UserID, now)
}

func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	rec, err := s.lookupRefresh(ctx, rawRefresh)
	if err != nil {
		// Idempotent logout: already invalid is OK.
		return nil
	}
	if err := s.refresh.Revoke(ctx, rec.ID, s.now()); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (UserDTO, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}
	return toDTO(user), nil
}

func (s *Service) ParseAccess(token string) (uuid.UUID, string, error) {
	return s.tokens.ParseAccessToken(token)
}

func (s *Service) lookupRefresh(ctx context.Context, raw string) (domain.RefreshToken, error) {
	if raw == "" {
		return domain.RefreshToken{}, domain.ErrUnauthorized
	}
	rec, err := s.refresh.FindByHash(ctx, hashToken(raw))
	if err != nil {
		return domain.RefreshToken{}, domain.ErrUnauthorized
	}
	if !rec.Active(s.now()) {
		return domain.RefreshToken{}, domain.ErrUnauthorized
	}
	return rec, nil
}

func (s *Service) issuePair(ctx context.Context, user domain.User) (TokenPair, error) {
	access, accessExp, err := s.tokens.IssueAccessToken(user.ID, user.Email)
	if err != nil {
		return TokenPair{}, err
	}
	raw, err := newOpaqueToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := s.now()
	exp := now.Add(s.refreshTTL)
	_, err = s.refresh.Create(ctx, domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(raw),
		ExpiresAt: exp,
		CreatedAt: now,
	})
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     raw,
		RefreshExpiresAt: exp,
		TokenType:        "Bearer",
	}, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
