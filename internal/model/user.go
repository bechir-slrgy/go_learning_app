package model

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) IsAdmin() bool { return r == RoleAdmin }

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

const minPasswordLen = 8

type UserInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (in *UserInput) ValidateProfile() error {
	in.Email = strings.TrimSpace(in.Email)
	in.Name = strings.TrimSpace(in.Name)

	if in.Email == "" {
		return fmt.Errorf("%w: email is required", ErrInvalid)
	}
	if _, err := mail.ParseAddress(in.Email); err != nil {
		return fmt.Errorf("%w: email is not a valid address", ErrInvalid)
	}
	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if len(in.Name) > 100 {
		return fmt.Errorf("%w: name must be 100 characters or fewer", ErrInvalid)
	}
	return nil
}

func (in *UserInput) Validate() error {
	if err := in.ValidateProfile(); err != nil {
		return err
	}

	if len(in.Password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalid, minPasswordLen)
	}
	if len(in.Password) > 72 {
		return fmt.Errorf("%w: password must be 72 characters or fewer", ErrInvalid)
	}
	return nil
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type TokenPair struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	TokenType       string `json:"token_type"`
	AccessExpiresIn int    `json:"access_expires_in"`
}
