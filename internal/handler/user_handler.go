package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type UserHandler struct {
	users *service.UserService
	auth  *Auth
}

func NewUserHandler(users *service.UserService, auth *Auth) *UserHandler {
	return &UserHandler{users: users, auth: auth}
}

func (h *UserHandler) Router() http.Handler {
	r := chi.NewRouter()

	r.Post("/", h.create)

	r.Group(func(r chi.Router) {
		r.Use(h.auth.RequireUser)

		r.Get("/", h.list)
		r.Get("/me", h.me)
		r.Get("/{id}", h.get)
		r.Put("/{id}", h.update)
		r.Delete("/{id}", h.delete)
	})
	return r
}

// me returns the full current user. The context carries only what the token
// claims (id, role, name), so email and created_at come from a fresh load.
// This is the one place the "trust claims" design pays a query, and it is a
// rare call, so that is fine.
func (h *UserHandler) me(w http.ResponseWriter, r *http.Request) {
	claims := userFrom(r.Context())
	u, err := h.users.Get(r.Context(), claims.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (h *UserHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, users)
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.UserInput](w, r)
	if !ok {
		return
	}
	u, err := h.users.Create(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, u)
}

func (h *UserHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	u, err := h.users.Get(r.Context(), id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	in, ok := decodeJSON[model.UserInput](w, r)
	if !ok {
		return
	}
	u, err := h.users.Update(r.Context(), user.ID, id, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (h *UserHandler) delete(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.users.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
