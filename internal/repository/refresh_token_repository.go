package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type RefreshTokenRepo struct {
	db *sql.DB
}

func NewRefreshTokenRepo(db *sql.DB) *RefreshTokenRepo {
	return &RefreshTokenRepo{db: db}
}

func (r *RefreshTokenRepo) Create(ctx context.Context, userID uuid.UUID, hash string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, hash, expiresAt)
	return err
}

func (r *RefreshTokenRepo) ByHash(ctx context.Context, hash string) (model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at
		 FROM refresh_tokens WHERE token_hash = $1 AND expires_at > now()`, hash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.RefreshToken{}, model.ErrUnauthorized
	}
	return t, err
}

func (r *RefreshTokenRepo) DeleteByHash(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, hash)
	return err
}

func (r *RefreshTokenRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	return err
}
