package handler

import (
	"net/http"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

func (s *Server) webhookList(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())

	hooks, err := s.hooks.List(r.Context(), user.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, hooks)
}

func (s *Server) webhookCreate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	in, ok := decodeJSON[model.WebhookInput](w, r)
	if !ok {
		return
	}
	hook, err := s.hooks.Create(r.Context(), user.ID, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, hook)
}

func (s *Server) webhookDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := s.hooks.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
