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

type UserWithToken struct {
	User
	Token string `json:"token"`
}

type UserInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (in *UserInput) Validate() error {
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
