package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

const userColumns = `id, email, name, role, created_at`

func scanUser(row scanner) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *UserRepo) ListByRole(ctx context.Context, role model.Role) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE role = $1 ORDER BY id`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// ByEmailWithHash returns the user and their bcrypt hash, for login only. The
// hash is a second return value, never a field on model.User, so it cannot
// leak into a response by accident. Not-found returns ErrUnauthorized, not
// ErrNotFound: "no such email" and "wrong password" must look identical to the
// caller or you leak which emails are registered.
func (r *UserRepo) ByEmailWithHash(ctx context.Context, email string) (model.User, string, error) {
	var u model.User
	var hash string
	err := r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+`, password_hash FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, "", model.ErrUnauthorized
	}
	return u, hash, err
}

func (r *UserRepo) List(ctx context.Context) ([]model.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Get(ctx context.Context, id uuid.UUID) (model.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, model.ErrNotFound
	}
	return u, err
}

func (r *UserRepo) Create(ctx context.Context, email, name, passwordHash string) (model.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING `+userColumns,
		email, name, passwordHash))
	if isUniqueViolation(err) {
		return model.User{}, model.ErrConflict
	}
	return u, err
}

func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, email, name string) (model.User, error) {
	u, err := scanUser(r.db.QueryRowContext(ctx,
		`UPDATE users SET email = $1, name = $2 WHERE id = $3 RETURNING `+userColumns,
		email, name, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, model.ErrNotFound
	}
	if isUniqueViolation(err) {
		return model.User{}, model.ErrConflict
	}
	return u, err
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
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
