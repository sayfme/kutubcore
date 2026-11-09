package handler

import (
	"net/http"

	"kutubcore/internal/domain/bookshop"
	"kutubcore/internal/domain/source"
	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
)

// GET /api/v1/bookshops — public.
func (h *Handler) ListBookshops(w http.ResponseWriter, r *http.Request) {
	limit, page := parsePage(r)
	shops, total, err := h.Dirs.ListBookshops(r.Context(), r.URL.Query().Get("q"), repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list bookshops.")
		return
	}
	if shops == nil {
		shops = []bookshop.Bookshop{}
	}
	writeJSON(w, http.StatusOK, shops, map[string]int{"limit": limit, "page": page, "total": total})
}

// GET /api/v1/bookshops/{id} — public.
func (h *Handler) GetBookshop(w http.ResponseWriter, r *http.Request) {
	shop, err := h.Dirs.GetBookshop(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, shop, nil)
}

// POST /api/v1/bookshops — organizer.
func (h *Handler) CreateBookshop(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var b bookshop.Bookshop
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.DirSvc.CreateBookshop(r.Context(), u.ID, &b)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// PATCH /api/v1/bookshops/{id} — organizer.
func (h *Handler) UpdateBookshop(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var b bookshop.Bookshop
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	b.ID = r.PathValue("id")
	updated, err := h.DirSvc.UpdateBookshop(r.Context(), u.ID, &b)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/bookshops/{id} — organizer (soft delete).
func (h *Handler) DeleteBookshop(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.DirSvc.DeleteBookshop(r.Context(), u.ID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/bookshops/{id}/links — organizer.
func (h *Handler) AddBookshopLink(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var l bookshop.Link
	if err := decodeJSON(r, &l); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.DirSvc.AddBookshopLink(r.Context(), r.PathValue("id"), &l)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// DELETE /api/v1/bookshops/{id}/links/{linkId} — organizer.
func (h *Handler) DeleteBookshopLink(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.Dirs.DeleteBookshopLink(r.Context(), r.PathValue("id"), r.PathValue("linkId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── sources ────────────────────────────────────────────────────────────────

// GET /api/v1/sources — public.
func (h *Handler) ListSources(w http.ResponseWriter, r *http.Request) {
	limit, page := parsePage(r)
	sources, total, err := h.Dirs.ListSources(r.Context(), r.URL.Query().Get("q"), repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list sources.")
		return
	}
	if sources == nil {
		sources = []source.Source{}
	}
	writeJSON(w, http.StatusOK, sources, map[string]int{"limit": limit, "page": page, "total": total})
}

// GET /api/v1/sources/{id} — public.
func (h *Handler) GetSource(w http.ResponseWriter, r *http.Request) {
	src, err := h.Dirs.GetSource(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src, nil)
}

// POST /api/v1/sources — organizer.
func (h *Handler) CreateSource(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var s source.Source
	if err := decodeJSON(r, &s); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.DirSvc.CreateSource(r.Context(), u.ID, &s)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// PATCH /api/v1/sources/{id} — organizer.
func (h *Handler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var s source.Source
	if err := decodeJSON(r, &s); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	s.ID = r.PathValue("id")
	updated, err := h.DirSvc.UpdateSource(r.Context(), u.ID, &s)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/sources/{id} — organizer (soft delete).
func (h *Handler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.DirSvc.DeleteSource(r.Context(), u.ID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/sources/{id}/links — organizer.
func (h *Handler) AddSourceLink(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var l source.Link
	if err := decodeJSON(r, &l); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.DirSvc.AddSourceLink(r.Context(), r.PathValue("id"), &l)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// DELETE /api/v1/sources/{id}/links/{linkId} — organizer.
func (h *Handler) DeleteSourceLink(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.Dirs.DeleteSourceLink(r.Context(), r.PathValue("id"), r.PathValue("linkId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
