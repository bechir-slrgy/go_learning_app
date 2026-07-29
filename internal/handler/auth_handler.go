package handler

import (
	"net/http"

	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.LoginInput](w, r)
	if !ok {
		return
	}
	pair, err := s.auth.Login(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, pair)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.RefreshInput](w, r)
	if !ok {
		return
	}
	pair, err := s.auth.Refresh(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, pair)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.RefreshInput](w, r)
	if !ok {
		return
	}
	if err := s.auth.Logout(r.Context(), in); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
