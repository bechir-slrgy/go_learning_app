package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// Router returns the /auth sub-router. All three routes are public: you cannot
// require a token to obtain a token.
func (h *AuthHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	return r
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.LoginInput](w, r)
	if !ok {
		return
	}
	pair, err := h.auth.Login(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, pair)
}

func (h *AuthHandler) refresh(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.RefreshInput](w, r)
	if !ok {
		return
	}
	pair, err := h.auth.Refresh(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, pair)
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.RefreshInput](w, r)
	if !ok {
		return
	}
	if err := h.auth.Logout(r.Context(), in); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
