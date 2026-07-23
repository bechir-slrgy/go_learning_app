package handler

import (
	"context"
	"net/http"
	"strings"

	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

// TokenVerifier is all the middleware needs: turn a raw JWT into the user it
// stands for. The concrete *auth.TokenService satisfies it. Verification is
// pure signature + expiry maths, so there is no database call per request.
type TokenVerifier interface {
	ParseAccess(raw string) (model.User, error)
}

type ctxKey struct{}

var userKey = ctxKey{}

type Auth struct {
	tokens TokenVerifier
}

func NewAuth(tokens TokenVerifier) *Auth {
	return &Auth{tokens: tokens}
}

// RequireUser is the authentication gate. It reads the bearer JWT, verifies it,
// and puts the user (built from the token's claims) on the request context.
func (a *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := bearerToken(r)
		if raw == "" {
			response.Unauthorized(w, "missing bearer token")
			return
		}

		user, err := a.tokens.ParseAccess(raw)
		if err != nil {
			// Every parse failure (bad signature, expired, wrong issuer) is a
			// 401 with the same body: never tell a caller why their token was
			// rejected in a way that helps forge a better one.
			response.Unauthorized(w, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is the authorization gate. It runs after RequireUser and reads
// the role straight off the context (i.e. from the verified token), so an
// admin check is one comparison and no query.
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
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

func userFrom(ctx context.Context) model.User {
	user, ok := ctx.Value(userKey).(model.User)
	if !ok {
		panic("no user in context: route is missing RequireUser middleware")
	}
	return user
}
