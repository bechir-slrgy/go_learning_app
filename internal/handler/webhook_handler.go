package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type WebhookHandler struct {
	hooks *service.WebhookService
	auth  *middleware.Auth
}

func NewWebhookHandler(hooks *service.WebhookService, auth *middleware.Auth) *WebhookHandler {
	return &WebhookHandler{hooks: hooks, auth: auth}
}

func (h *WebhookHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(h.auth.RequireUser)

	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *WebhookHandler) list(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	hooks, err := h.hooks.List(r.Context(), user.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, hooks)
}

func (h *WebhookHandler) create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	in, ok := decodeJSON[model.WebhookInput](w, r)
	if !ok {
		return
	}
	hook, err := h.hooks.Create(r.Context(), user.ID, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, hook)
}

func (h *WebhookHandler) delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.hooks.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
