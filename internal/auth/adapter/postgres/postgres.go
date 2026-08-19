package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/Brohammad/BugSathi/internal/auth/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	const q = `
		INSERT INTO users (id, email, password_hash, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, email, password_hash, name, created_at, updated_at`
	var out domain.User
	err := r.pool.QueryRow(ctx, q,
		user.ID, user.Email, user.PasswordHash, user.Name, user.CreatedAt, user.UpdatedAt,
	).Scan(&out.ID, &out.Email, &out.PasswordHash, &out.Name, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, domain.ErrEmailTaken
		}
		return domain.User{}, err
	}
	return out, nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE email = $1`
	var out domain.User
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&out.ID, &out.Email, &out.PasswordHash, &out.Name, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return out, err
}

func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, name, created_at, updated_at
		FROM users WHERE id = $1`
	var out domain.User
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.Email, &out.PasswordHash, &out.Name, &out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	return out, err
}

type RefreshRepo struct {
	pool *pgxpool.Pool
}

func NewRefreshRepo(pool *pgxpool.Pool) *RefreshRepo {
	return &RefreshRepo{pool: pool}
}

func (r *RefreshRepo) Create(ctx context.Context, token domain.RefreshToken) (domain.RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at`
	var out domain.RefreshToken
	err := r.pool.QueryRow(ctx, q,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt, token.RevokedAt, token.CreatedAt,
	).Scan(&out.ID, &out.UserID, &out.TokenHash, &out.ExpiresAt, &out.RevokedAt, &out.CreatedAt)
	return out, err
}

func (r *RefreshRepo) FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`
	var out domain.RefreshToken
	err := r.pool.QueryRow(ctx, q, hash).Scan(
		&out.ID, &out.UserID, &out.TokenHash, &out.ExpiresAt, &out.RevokedAt, &out.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	return out, err
}

func (r *RefreshRepo) Rotate(ctx context.Context, hash string, at time.Time, replacement domain.RefreshToken) (domain.RefreshToken, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.RefreshToken{}, err
	}
	defer tx.Rollback(ctx)

	var consumed domain.RefreshToken
	err = tx.QueryRow(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at`,
		hash, at,
	).Scan(&consumed.ID, &consumed.UserID, &consumed.TokenHash, &consumed.ExpiresAt, &consumed.RevokedAt, &consumed.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.RefreshToken{}, err
	}

	replacement.UserID = consumed.UserID
	const insert = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at`
	if _, err := tx.Exec(ctx, insert,
		replacement.ID, replacement.UserID, replacement.TokenHash, replacement.ExpiresAt, replacement.RevokedAt, replacement.CreatedAt,
	); err != nil {
		return domain.RefreshToken{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RefreshToken{}, err
	}
	return consumed, nil
}

func (r *RefreshRepo) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`
	ct, err := r.pool.Exec(ctx, q, id, at)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RefreshRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	const q = `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, q, userID, at)
	return err
}
