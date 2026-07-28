package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type AdminHandler struct {
	tasks *service.TaskService
	auth  *middleware.Auth
}

func NewAdminHandler(tasks *service.TaskService, auth *middleware.Auth) *AdminHandler {
	return &AdminHandler{tasks: tasks, auth: auth}
}

func (h *AdminHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(h.auth.RequireUser)
	r.Use(h.auth.RequireAdmin)

	r.Get("/tasks", h.queue)
	r.Post("/tasks/{id}/approve", h.approve)
	r.Post("/tasks/{id}/reject", h.reject)
	return r
}

func (h *AdminHandler) queue(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())

	status := model.StatusSubmitted
	if raw := r.URL.Query().Get("status"); raw != "" {
		status = model.TaskStatus(raw)
	}

	tasks, err := h.tasks.ReviewQueue(r.Context(), admin, status)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, tasks)
}

func (h *AdminHandler) approve(w http.ResponseWriter, r *http.Request) {
	h.review(w, r, model.StatusApproved)
}

func (h *AdminHandler) reject(w http.ResponseWriter, r *http.Request) {
	h.review(w, r, model.StatusRejected)
}

func (h *AdminHandler) review(w http.ResponseWriter, r *http.Request, decision model.TaskStatus) {
	admin := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := h.tasks.Review(r.Context(), admin, id, decision)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}
