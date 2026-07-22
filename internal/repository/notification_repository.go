package repository

import (
	"context"
	"database/sql"

	"task_crud_api/internal/model"
)

type NotificationRepo struct {
	db *sql.DB
}

func NewNotificationRepo(db *sql.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

const notificationColumns = `id, user_id, task_id, message, read, created_at`

func scanNotification(row scanner) (model.Notification, error) {
	var n model.Notification
	err := row.Scan(&n.ID, &n.UserID, &n.TaskID, &n.Message, &n.Read, &n.CreatedAt)
	return n, err
}

func (r *NotificationRepo) List(ctx context.Context, userID int) ([]model.Notification, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+notificationColumns+` FROM notifications
		 WHERE user_id = $1 ORDER BY read ASC, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NotificationRepo) Create(ctx context.Context, userID, taskID int, message string) (model.Notification, error) {
	return scanNotification(r.db.QueryRowContext(ctx,
		`INSERT INTO notifications (user_id, task_id, message) VALUES ($1, $2, $3)
		 RETURNING `+notificationColumns, userID, taskID, message))
}

func (r *NotificationRepo) MarkRead(ctx context.Context, userID, id int) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`, id, userID)
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
