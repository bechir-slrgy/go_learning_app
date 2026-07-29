package handler

import (
	"net/http"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/response"
)

func (s *Server) noteList(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	notes, err := s.notes.List(r.Context(), user.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, notes)
}

func (s *Server) noteMarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := s.notes.MarkRead(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
