package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
	"task_crud_api/internal/service"
)

type TaskHandler struct {
	tasks *service.TaskService
	auth  *middleware.Auth
}

func NewTaskHandler(tasks *service.TaskService, auth *middleware.Auth) *TaskHandler {
	return &TaskHandler{tasks: tasks, auth: auth}
}

func (h *TaskHandler) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(h.auth.RequireUser)

	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Post("/import", h.importTasks)
	r.Get("/{id}", h.get)
	r.Put("/{id}", h.update)
	r.Post("/{id}/submit", h.submit)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *TaskHandler) list(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	tasks, err := h.tasks.List(r.Context(), user.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, tasks)
}

func (h *TaskHandler) create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	in, ok := decodeJSON[model.TaskInput](w, r)
	if !ok {
		return
	}
	t, err := h.tasks.Create(r.Context(), user.ID, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, t)
}

func (h *TaskHandler) importTasks(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	limit := 5
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "limit must be a number")
			return
		}
		limit = n
	}

	tasks, err := h.tasks.Import(r.Context(), user.ID, limit)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, tasks)
}

func (h *TaskHandler) get(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := h.tasks.Get(r.Context(), user.ID, id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	in, ok := decodeJSON[model.TaskInput](w, r)
	if !ok {
		return
	}
	t, err := h.tasks.Update(r.Context(), user.ID, id, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) submit(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := h.tasks.Submit(r.Context(), user, id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (h *TaskHandler) delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.tasks.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
