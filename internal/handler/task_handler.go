package handler

import (
	"net/http"
	"strconv"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

func (s *Server) taskList(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	page := queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := queryInt(r, "page_size", 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	res, err := s.tasks.List(r.Context(), user.ID, page, pageSize)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, res)
}

func (s *Server) taskCreate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	in, ok := decodeJSON[model.TaskInput](w, r)
	if !ok {
		return
	}
	t, err := s.tasks.Create(r.Context(), user.ID, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, t)
}

func (s *Server) taskImport(w http.ResponseWriter, r *http.Request) {
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

	tasks, err := s.tasks.Import(r.Context(), user.ID, limit)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, tasks)
}

func (s *Server) taskGet(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := s.tasks.Get(r.Context(), user.ID, id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (s *Server) taskUpdate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	in, ok := decodeJSON[model.TaskInput](w, r)
	if !ok {
		return
	}
	t, err := s.tasks.Update(r.Context(), user.ID, id, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (s *Server) taskSubmit(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := s.tasks.Submit(r.Context(), user, id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}

func (s *Server) taskDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := s.tasks.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
