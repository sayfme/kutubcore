package handler

import (
	"net/http"

	"kutubcore/internal/domain/writer"
	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
)

// GET /api/v1/writers — public.
func (h *Handler) ListWriters(w http.ResponseWriter, r *http.Request) {
	limit, page := parsePage(r)
	writers, total, err := h.Writers.ListWriters(r.Context(), r.URL.Query().Get("q"), repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list writers.")
		return
	}
	if writers == nil {
		writers = []writer.Writer{}
	}
	writeJSON(w, http.StatusOK, writers, map[string]int{"limit": limit, "page": page, "total": total})
}

// GET /api/v1/writers/{id} — public.
func (h *Handler) GetWriter(w http.ResponseWriter, r *http.Request) {
	wr, err := h.Writers.GetWriter(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wr, nil)
}

// POST /api/v1/writers — organizer.
func (h *Handler) CreateWriter(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var wr writer.Writer
	if err := decodeJSON(r, &wr); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.Catalog.CreateWriter(r.Context(), u.ID, &wr)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// PATCH /api/v1/writers/{id} — organizer.
func (h *Handler) UpdateWriter(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var wr writer.Writer
	if err := decodeJSON(r, &wr); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	wr.ID = r.PathValue("id")
	updated, err := h.Catalog.UpdateWriter(r.Context(), u.ID, &wr)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/writers/{id} — organizer (soft delete).
func (h *Handler) DeleteWriter(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.Catalog.DeleteWriter(r.Context(), u.ID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
