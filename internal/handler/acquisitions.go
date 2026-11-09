package handler

import (
	"net/http"

	"kutubcore/internal/domain/acquisition"
	"kutubcore/internal/repository"
)

// All acquisition endpoints are per-user; user identity comes from the session.

// GET /api/v1/me/acquisitions — authenticated.
func (h *Handler) ListAcquisitions(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	limit, page := parsePage(r)
	list, total, err := h.Acquisitions.ListAcquisitions(r.Context(), userID, repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list acquisitions.")
		return
	}
	if list == nil {
		list = []acquisition.Acquisition{}
	}
	writeJSON(w, http.StatusOK, list, map[string]int{"limit": limit, "page": page, "total": total})
}

// POST /api/v1/me/acquisitions — authenticated.
func (h *Handler) CreateAcquisition(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var a acquisition.Acquisition
	if err := decodeJSON(r, &a); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.AcquisitionSvc.Create(r.Context(), userID, &a)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// GET /api/v1/me/acquisitions/{id} — authenticated (owner only).
func (h *Handler) GetAcquisition(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	a, err := h.Acquisitions.GetAcquisition(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a, nil)
}

// DELETE /api/v1/me/acquisitions/{id} — authenticated (owner only).
// Deletes the acquisition with its items and copies (cascades).
func (h *Handler) DeleteAcquisition(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	if err := h.Acquisitions.DeleteAcquisition(r.Context(), userID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/me/copies?book_id= — authenticated (physical/digital inventory).
func (h *Handler) ListCopies(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	limit, page := parsePage(r)
	copies, total, err := h.Acquisitions.ListCopies(r.Context(), userID, r.URL.Query().Get("book_id"), repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list copies.")
		return
	}
	if copies == nil {
		copies = []acquisition.Copy{}
	}
	writeJSON(w, http.StatusOK, copies, map[string]int{"limit": limit, "page": page, "total": total})
}
