package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/google/uuid"
)

type UserRepo struct {
	mu    sync.Mutex
	byID  map[uuid.UUID]domain.User
	email map[string]uuid.UUID
}

func NewUserRepo() *UserRepo {
	return &UserRepo{
		byID:  make(map[uuid.UUID]domain.User),
		email: make(map[string]uuid.UUID),
	}
}

func (r *UserRepo) Create(_ context.Context, user domain.User) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.email[user.Email]; ok {
		return domain.User{}, domain.ErrEmailTaken
	}
	r.byID[user.ID] = user
	r.email[user.Email] = user.ID
	return user, nil
}

func (r *UserRepo) FindByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.email[email]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *UserRepo) FindByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

type RefreshRepo struct {
	mu   sync.Mutex
	byID map[uuid.UUID]domain.RefreshToken
	hash map[string]uuid.UUID
}

func NewRefreshRepo() *RefreshRepo {
	return &RefreshRepo{
		byID: make(map[uuid.UUID]domain.RefreshToken),
		hash: make(map[string]uuid.UUID),
	}
}

func (r *RefreshRepo) Create(_ context.Context, token domain.RefreshToken) (domain.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[token.ID] = token
	r.hash[token.TokenHash] = token.ID
	return token, nil
}

func (r *RefreshRepo) FindByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.hash[hash]
	if !ok {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *RefreshRepo) Revoke(_ context.Context, id uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	t.RevokedAt = &at
	r.byID[id] = t
	return nil
}

func (r *RefreshRepo) RevokeAllForUser(_ context.Context, userID uuid.UUID, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.byID {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &at
			r.byID[id] = t
		}
	}
	return nil
}
