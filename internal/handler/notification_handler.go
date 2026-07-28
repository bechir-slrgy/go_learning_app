package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type NotificationHandler struct {
	notes *service.NotificationService
	auth  *middleware.Auth
}

func NewNotificationHandler(notes *service.NotificationService, auth *middleware.Auth) *NotificationHandler {
	return &NotificationHandler{notes: notes, auth: auth}
}

func (h *NotificationHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(h.auth.RequireUser)

	r.Get("/", h.list)
	r.Post("/{id}/read", h.markRead)
	return r
}

func (h *NotificationHandler) list(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	notes, err := h.notes.List(r.Context(), user.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, notes)
}

func (h *NotificationHandler) markRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.notes.MarkRead(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
