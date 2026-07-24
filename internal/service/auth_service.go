package service

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"task_crud_api/internal/auth"

	"task_crud_api/internal/model"
)

type Credentialer interface {
	ByEmailWithHash(ctx context.Context, email string) (model.User, string, error)
	Get(ctx context.Context, id uuid.UUID) (model.User, error)
}

type RefreshStore interface {
	Create(ctx context.Context, userID uuid.UUID, hash string, expiresAt time.Time) error
	ByHash(ctx context.Context, hash string) (model.RefreshToken, error)
	DeleteByHash(ctx context.Context, hash string) error
	DeleteForUser(ctx context.Context, userID uuid.UUID) error
}

type Tokens interface {
	IssueAccess(u model.User, now time.Time) (string, time.Time, error)
	NewRefreshToken() (plain, hash string, expiresAt time.Time, err error)
}

type AuthService struct {
	users   Credentialer
	refresh RefreshStore
	tokens  Tokens
}

func NewAuthService(users Credentialer, refresh RefreshStore, tokens Tokens) *AuthService {
	return &AuthService{users: users, refresh: refresh, tokens: tokens}
}

func (s *AuthService) Login(ctx context.Context, in model.LoginInput) (model.TokenPair, error) {
	user, hash, err := s.users.ByEmailWithHash(ctx, in.Email)
	if err != nil {
		return model.TokenPair{}, model.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		return model.TokenPair{}, model.ErrUnauthorized
	}
	return s.issuePair(ctx, user)
}

func (s *AuthService) Refresh(ctx context.Context, in model.RefreshInput) (model.TokenPair, error) {
	hash := auth.HashRefreshToken(in.RefreshToken)

	stored, err := s.refresh.ByHash(ctx, hash)
	if err != nil {
		return model.TokenPair{}, model.ErrUnauthorized
	}

	if err := s.refresh.DeleteByHash(ctx, hash); err != nil {
		return model.TokenPair{}, err
	}

	user, err := s.users.Get(ctx, stored.UserID)
	if err != nil {
		return model.TokenPair{}, model.ErrUnauthorized
	}
	return s.issuePair(ctx, user)
}

func (s *AuthService) Logout(ctx context.Context, in model.RefreshInput) error {
	return s.refresh.DeleteByHash(ctx, auth.HashRefreshToken(in.RefreshToken))
}

func (s *AuthService) issuePair(ctx context.Context, user model.User) (model.TokenPair, error) {
	now := time.Now()

	access, accessExp, err := s.tokens.IssueAccess(user, now)
	if err != nil {
		return model.TokenPair{}, err
	}

	plain, hash, refreshExp, err := s.tokens.NewRefreshToken()
	if err != nil {
		return model.TokenPair{}, err
	}
	if err := s.refresh.Create(ctx, user.ID, hash, refreshExp); err != nil {
		return model.TokenPair{}, err
	}

	return model.TokenPair{
		AccessToken:     access,
		RefreshToken:    plain,
		TokenType:       "Bearer",
		AccessExpiresIn: int(time.Until(accessExp).Seconds()),
	}, nil
}
