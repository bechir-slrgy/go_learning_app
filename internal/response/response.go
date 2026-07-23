package response

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task_crud_api/internal/model"
)

type ResponseError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ResponseError{Status: status, Message: message})
}


func Unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	Error(w, http.StatusUnauthorized, message)
}

func ErrorFrom(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, model.ErrInvalid):
		Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, model.ErrUnauthorized):
		Unauthorized(w, "unauthorized")
	case errors.Is(err, model.ErrForbidden):
		Error(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, model.ErrNotFound):
		Error(w, http.StatusNotFound, "not found")
	case errors.Is(err, model.ErrConflict):
		Error(w, http.StatusConflict, "already exists")
	case errors.Is(err, model.ErrUpstream):
		Error(w, http.StatusBadGateway, "upstream service unavailable")
	default:
		log.Printf("unhandled error: %v", err)
		Error(w, http.StatusInternalServerError, "internal error")
	}
}
