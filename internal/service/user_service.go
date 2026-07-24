package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type UserRepo interface {
	List(ctx context.Context) ([]model.User, error)
	Get(ctx context.Context, id uuid.UUID) (model.User, error)
	Create(ctx context.Context, email, name, passwordHash string) (model.User, error)
	Update(ctx context.Context, id uuid.UUID, email, name string) (model.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserService struct {
	repo UserRepo
}

func NewUserService(repo UserRepo) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) List(ctx context.Context) ([]model.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) Get(ctx context.Context, id uuid.UUID) (model.User, error) {
	return s.repo.Get(ctx, id)
}

func (s *UserService) Create(ctx context.Context, in model.UserInput) (model.User, error) {
	if err := in.Validate(); err != nil {
		return model.User{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}
	return s.repo.Create(ctx, in.Email, in.Name, string(hash))
}

func (s *UserService) Update(ctx context.Context, callerID, id uuid.UUID, in model.UserInput) (model.User, error) {
	if callerID != id {
		return model.User{}, model.ErrForbidden
	}
	if err := in.ValidateProfile(); err != nil {
		return model.User{}, err
	}
	return s.repo.Update(ctx, id, in.Email, in.Name)
}

func (s *UserService) Delete(ctx context.Context, callerID, id uuid.UUID) error {
	if callerID != id {
		return model.ErrForbidden
	}
	return s.repo.Delete(ctx, id)
}
