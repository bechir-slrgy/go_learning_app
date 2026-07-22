package handler

import (
	"context"
	"net/http"
	"strings"

	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

type Authenticator interface {
	ByToken(ctx context.Context, token string) (model.User, error)
}

type ctxKey struct{}

var userKey = ctxKey{}

type Auth struct {
	users Authenticator
}

func NewAuth(users Authenticator) *Auth {
	return &Auth{users: users}
}

func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			response.ErrorFrom(w, model.ErrUnauthorized)
			return
		}

		user, err := a.users.ByToken(r.Context(), token)
		if err != nil {
			response.ErrorFrom(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !userFrom(r.Context()).Role.IsAdmin() {
			response.ErrorFrom(w, model.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

func userFrom(ctx context.Context) model.User {
	user, ok := ctx.Value(userKey).(model.User)
	if !ok {
		panic("no user in context: route is missing RequireUser middleware")
	}
	return user
}
