package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type TaskRepo struct {
	db *sql.DB
}

func NewTaskRepo(db *sql.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

const taskColumns = `id, user_id, title, status, reviewed_by, reviewed_at, created_at`

func scanTask(row scanner) (model.Task, error) {
	var t model.Task

	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Status,
		&t.ReviewedBy, &t.ReviewedAt, &t.CreatedAt)
	return t, err
}

func (r *TaskRepo) ListPaged(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE user_id = $1 ORDER BY created_at DESC, id DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return collectTasks(rows)
}

func (r *TaskRepo) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM tasks WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

func (r *TaskRepo) Get(ctx context.Context, userID, id uuid.UUID) (model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1 AND user_id = $2`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	return t, err
}

func (r *TaskRepo) Create(ctx context.Context, userID uuid.UUID, title string) (model.Task, error) {
	return scanTask(r.db.QueryRowContext(ctx,
		`INSERT INTO tasks (user_id, title) VALUES ($1, $2) RETURNING `+taskColumns,
		userID, title))
}

func (r *TaskRepo) Update(ctx context.Context, userID, id uuid.UUID, title string) (model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`UPDATE tasks SET title = $1 WHERE id = $2 AND user_id = $3 RETURNING `+taskColumns,
		title, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	return t, err
}

func (r *TaskRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM tasks WHERE id = $1 AND user_id = $2`, id, userID)
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

func (r *TaskRepo) GetAny(ctx context.Context, id uuid.UUID) (model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	return t, err
}

func (r *TaskRepo) ListByStatus(ctx context.Context, status model.TaskStatus) ([]model.Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE status = $1 ORDER BY id`, status)
	if err != nil {
		return nil, err
	}
	return collectTasks(rows)
}

func (r *TaskRepo) SetStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus) (model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`UPDATE tasks SET status = $1, reviewed_by = NULL, reviewed_at = NULL
		 WHERE id = $2 RETURNING `+taskColumns, status, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	return t, err
}

func (r *TaskRepo) SetReviewed(ctx context.Context, id uuid.UUID, status model.TaskStatus, reviewerID uuid.UUID) (model.Task, error) {
	t, err := scanTask(r.db.QueryRowContext(ctx,
		`UPDATE tasks SET status = $1, reviewed_by = $2, reviewed_at = now()
		 WHERE id = $3 RETURNING `+taskColumns, status, reviewerID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	return t, err
}

func collectTasks(rows *sql.Rows) ([]model.Task, error) {
	defer rows.Close()

	tasks := []model.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
