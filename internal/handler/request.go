package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"task_crud_api/internal/response"
)

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		response.Error(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return v, false
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&v); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return v, false
	}
	return v, true
}

func parseID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		response.Error(w, http.StatusBadRequest, "id must be a positive number")
		return 0, false
	}
	return id, true
}
