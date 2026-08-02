package jwtmgr

import (
	"fmt"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func New(secret string, ttl time.Duration) (*Manager, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return &Manager{
		secret: []byte(secret),
		ttl:    ttl,
		now:    time.Now,
	}, nil
}

type claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

func (m *Manager) IssueAccessToken(userID uuid.UUID, email string) (string, time.Time, error) {
	now := m.now()
	exp := now.Add(m.ttl)
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "bugsathi",
		},
	})
	signed, err := t.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

func (m *Manager) ParseAccessToken(token string) (uuid.UUID, string, error) {
	parsed, err := jwt.ParseWithClaims(token, &claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, domain.ErrUnauthorized
		}
		return m.secret, nil
	})
	if err != nil || !parsed.Valid {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || c.Subject == "" {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, "", domain.ErrUnauthorized
	}
	return id, c.Email, nil
}
