package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"task_crud_api/internal/model"
)

type UserRepo interface {
	ByToken(ctx context.Context, token string) (model.User, error)
	List(ctx context.Context) ([]model.User, error)
	Get(ctx context.Context, id int) (model.User, error)
	Create(ctx context.Context, email, name, token string) (model.User, error)
	Update(ctx context.Context, id int, email, name string) (model.User, error)
	Delete(ctx context.Context, id int) error
}

type UserService struct {
	repo UserRepo
}

func NewUserService(repo UserRepo) *UserService {
	return &UserService{repo: repo}
}

func newToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *UserService) ByToken(ctx context.Context, token string) (model.User, error) {
	return s.repo.ByToken(ctx, token)
}

func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) Get(ctx context.Context, id int) (model.User, error) {
	return s.repo.Get(ctx, id)
}

func (s *UserService) Create(ctx context.Context, in model.UserInput) (model.UserWithToken, error) {
	if err := in.Validate(); err != nil {
		return model.UserWithToken{}, err
	}
	token, err := newToken()
	if err != nil {
		return model.UserWithToken{}, err
	}
	u, err := s.repo.Create(ctx, in.Email, in.Name, token)
	if err != nil {
		return model.UserWithToken{}, err
	}
	return model.UserWithToken{User: u, Token: token}, nil
}

func (s *UserService) Update(ctx context.Context, callerID, id int, in model.UserInput) (model.User, error) {
	if callerID != id {
		return model.User{}, model.ErrForbidden
	}
	if err := in.Validate(); err != nil {
		return model.User{}, err
	}
	return s.repo.Update(ctx, id, in.Email, in.Name)
}

func (s *UserService) Delete(ctx context.Context, callerID, id int) error {
	if callerID != id {
		return model.ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}
