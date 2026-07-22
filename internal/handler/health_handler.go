package handler

import (
	"net/http"

	"task_crud_api/internal/response"
)

func Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
