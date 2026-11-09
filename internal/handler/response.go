package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"kutubcore/internal/repository"
	"kutubcore/internal/service"
)

// writeJSON writes the Blueprint §9 success envelope.
func writeJSON(w http.ResponseWriter, status int, data any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data, "meta": meta})
}

// writeError writes the Blueprint §9 error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   map[string]string{"code": code, "message": message},
	})
}

// writeServiceError maps domain errors to HTTP status + code.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "The requested record was not found.")
	case errors.Is(err, service.ErrDuplicate):
		writeError(w, http.StatusConflict, "DUPLICATE_POSSIBLE", err.Error())
	case errors.Is(err, service.ErrValidation):
		msg := err.Error()
		// Strip prefix for a friendlier message, keep full detail in code path.
		msg = strings.TrimPrefix(msg, "validation error: ")
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg)
	case errors.Is(err, service.ErrInvalidPassword):
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid identifier or password.")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Something went wrong.")
	}
}

func decodeJSON(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1 MiB cap
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func parsePage(r *http.Request) (limit, page int) {
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	return limit, page
}
