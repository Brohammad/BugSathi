package port

import (
	"context"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	Create(ctx context.Context, user domain.User) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, token domain.RefreshToken) (domain.RefreshToken, error)
	FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
	// Rotate atomically revokes the active token identified by hash and inserts replacement.
	// Returns the consumed token on success; ErrUnauthorized when missing, expired, or already redeemed.
	Rotate(ctx context.Context, hash string, at time.Time, replacement domain.RefreshToken) (domain.RefreshToken, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenManager interface {
	IssueAccessToken(userID uuid.UUID, email string) (token string, expiresAt time.Time, err error)
	ParseAccessToken(token string) (userID uuid.UUID, email string, err error)
}
