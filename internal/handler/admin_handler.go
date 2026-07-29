package handler

import (
	"net/http"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

func (s *Server) adminQueue(w http.ResponseWriter, r *http.Request) {
	admin := middleware.UserFrom(r.Context())

	status := model.StatusSubmitted
	if raw := r.URL.Query().Get("status"); raw != "" {
		status = model.TaskStatus(raw)
	}

	tasks, err := s.tasks.ReviewQueue(r.Context(), admin, status)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, tasks)
}

func (s *Server) adminApprove(w http.ResponseWriter, r *http.Request) {
	s.adminReview(w, r, model.StatusApproved)
}

func (s *Server) adminReject(w http.ResponseWriter, r *http.Request) {
	s.adminReview(w, r, model.StatusRejected)
}

func (s *Server) adminReview(w http.ResponseWriter, r *http.Request, decision model.TaskStatus) {
	admin := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	t, err := s.tasks.Review(r.Context(), admin, id, decision)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, t)
}
