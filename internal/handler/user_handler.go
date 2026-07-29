package handler

import (
	"net/http"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/model"
	"task_crud_api/internal/response"
)

func (s *Server) userMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.UserFrom(r.Context())
	u, err := s.users.Get(r.Context(), claims.ID)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (s *Server) userList(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.List(r.Context())
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, users)
}

func (s *Server) userCreate(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeJSON[model.UserInput](w, r)
	if !ok {
		return
	}
	u, err := s.users.Create(r.Context(), in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}

	response.JSON(w, http.StatusCreated, u)
}

func (s *Server) userGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	u, err := s.users.Get(r.Context(), id)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (s *Server) userUpdate(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	in, ok := decodeJSON[model.UserInput](w, r)
	if !ok {
		return
	}
	u, err := s.users.Update(r.Context(), user.ID, id, in)
	if err != nil {
		response.ErrorFrom(w, err)
		return
	}
	response.JSON(w, http.StatusOK, u)
}

func (s *Server) userDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFrom(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := s.users.Delete(r.Context(), user.ID, id); err != nil {
		response.ErrorFrom(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
