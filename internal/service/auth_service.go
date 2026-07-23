package service

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"task_crud_api/internal/auth"
	"task_crud_api/internal/model"
)

// Credentialer is the slice of the user repository this service needs.
type Credentialer interface {
	ByEmailWithHash(ctx context.Context, email string) (model.User, string, error)
	Get(ctx context.Context, id int) (model.User, error)
}

// RefreshStore persists refresh-token hashes so they can be checked and revoked.
type RefreshStore interface {
	Create(ctx context.Context, userID int, hash string, expiresAt time.Time) error
	ByHash(ctx context.Context, hash string) (model.RefreshToken, error)
	DeleteByHash(ctx context.Context, hash string) error
	DeleteForUser(ctx context.Context, userID int) error
}

// Tokens is the token machinery, declared as an interface so the service does
// not import a concrete signer.
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

// Login verifies the password and issues a token pair.
//
// A wrong email and a wrong password return the SAME error (ErrUnauthorized),
// so an attacker cannot tell which emails are registered. bcrypt's compare is
// constant-time, and even the not-found path should ideally cost the same, but
// the shared 401 is the load-bearing part.
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

// Refresh rotates the token: the old refresh token is spent and a new pair is
// issued. Rotation means a stolen refresh token works at most once before the
// legitimate client's next refresh invalidates it.
func (s *AuthService) Refresh(ctx context.Context, in model.RefreshInput) (model.TokenPair, error) {
	hash := auth.HashRefreshToken(in.RefreshToken)

	stored, err := s.refresh.ByHash(ctx, hash)
	if err != nil {
		return model.TokenPair{}, model.ErrUnauthorized
	}

	// Spend the old token first, so a replay cannot mint a second pair.
	if err := s.refresh.DeleteByHash(ctx, hash); err != nil {
		return model.TokenPair{}, err
	}

	user, err := s.users.Get(ctx, stored.UserID)
	if err != nil {
		// The user was deleted after the token was issued.
		return model.TokenPair{}, model.ErrUnauthorized
	}
	return s.issuePair(ctx, user)
}

// Logout spends one refresh token. It is deliberately quiet: an unknown token
// is not an error, because logging out something already gone is a success
// from the caller's point of view.
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
