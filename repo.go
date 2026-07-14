package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNotFound = errors.New("task not found")

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

const taskColumns = `id, title, done, created_at`

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.Title, &t.Done, &t.CreatedAt)
	return t, err
}

func (r *Repo) List(ctx context.Context) ([]Task, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+taskColumns+` FROM tasks ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (r *Repo) Get(ctx context.Context, id int) (Task, error) {
	t, err := scanTask(r.pool.QueryRow(ctx,
		`SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, errNotFound
	}
	return t, err
}

func (r *Repo) Add(ctx context.Context, title string) (Task, error) {
	return scanTask(r.pool.QueryRow(ctx,
		`INSERT INTO tasks (title) VALUES ($1) RETURNING `+taskColumns, title))
}

func (r *Repo) Update(ctx context.Context, id int, title string, done bool) (Task, error) {
	t, err := scanTask(r.pool.QueryRow(ctx,
		`UPDATE tasks SET title = $1, done = $2 WHERE id = $3 RETURNING `+taskColumns,
		title, done, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, errNotFound
	}
	return t, err
}

func (r *Repo) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}
