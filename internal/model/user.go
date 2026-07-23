package model

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

func (r Role) IsAdmin() bool { return r == RoleAdmin }

type User struct {
	ID        int       `json:"id"`
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
	// The password is not trimmed: leading and trailing spaces are legitimate
	// password characters, and bcrypt caps the input at 72 bytes anyway.
	if len(in.Password) < minPasswordLen {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalid, minPasswordLen)
	}
	if len(in.Password) > 72 {
		return fmt.Errorf("%w: password must be 72 characters or fewer", ErrInvalid)
	}
	return nil
}

// LoginInput is the credentials POSTed to /api/login. Deliberately not
// validated for length: telling an attacker "that is too short to be our
// password" leaks the rule. A wrong login is always a plain 401.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshInput and its sibling carry the opaque refresh token.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenPair is what login and refresh return. AccessExpiresIn is seconds, so a
// client knows when to refresh without parsing the JWT.
type TokenPair struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	TokenType       string `json:"token_type"`
	AccessExpiresIn int    `json:"access_expires_in"`
}
