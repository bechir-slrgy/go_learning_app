package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type WebhookRepo struct {
	db *sql.DB
}

func NewWebhookRepo(db *sql.DB) *WebhookRepo {
	return &WebhookRepo{db: db}
}

const webhookColumns = `id, user_id, url, created_at`

func scanWebhook(row scanner) (model.Webhook, error) {
	var w model.Webhook
	err := row.Scan(&w.ID, &w.UserID, &w.URL, &w.CreatedAt)
	return w, err
}

func (r *WebhookRepo) List(ctx context.Context, userID uuid.UUID) ([]model.Webhook, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+webhookColumns+` FROM webhooks WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hooks := []model.Webhook{}
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, w)
	}
	return hooks, rows.Err()
}

func (r *WebhookRepo) Create(ctx context.Context, userID uuid.UUID, url string) (model.Webhook, error) {
	return scanWebhook(r.db.QueryRowContext(ctx,
		`INSERT INTO webhooks (user_id, url) VALUES ($1, $2) RETURNING `+webhookColumns,
		userID, url))
}

func (r *WebhookRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM webhooks WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return model.ErrNotFound
	}
	return nil
}
