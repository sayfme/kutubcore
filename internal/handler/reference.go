package handler

import (
	"net/http"

	"kutubcore/internal/repository"
)

// GET /api/v1/languages — public.
func (h *Handler) ListLanguages(w http.ResponseWriter, r *http.Request) {
	langs, err := h.Ref.ListLanguages(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list languages.")
		return
	}
	if langs == nil {
		langs = []repository.Language{}
	}
	writeJSON(w, http.StatusOK, langs, nil)
}

// GET /api/v1/genres — public.
func (h *Handler) ListGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.Ref.ListGenres(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list genres.")
		return
	}
	if genres == nil {
		genres = []repository.Genre{}
	}
	writeJSON(w, http.StatusOK, genres, nil)
}
