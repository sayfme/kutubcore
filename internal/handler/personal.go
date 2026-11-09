package handler

import (
	"net/http"

	"kutubcore/internal/domain/collection"
	"kutubcore/internal/domain/reading"
	"kutubcore/internal/domain/review"
	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
)

// All /me routes below run after SessionAuth; RequireAuth is applied by
// the router so handlers can assume a user is present.
//
// Ownership: user identity comes from the session cookie, never the client.

func meUser(w http.ResponseWriter, r *http.Request) (id string, ok bool) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return "", false
	}
	return u.ID, true
}

// GET /api/v1/me/dashboard — authenticated.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	d, err := h.Personal.Dashboard(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to load dashboard.")
		return
	}
	if d.RecentSessions == nil {
		d.RecentSessions = []reading.Session{}
	}
	if d.RecentlyAdded == nil {
		d.RecentlyAdded = []collection.UserBook{}
	}
	writeJSON(w, http.StatusOK, d, nil)
}

// GET /api/v1/me/books — authenticated.
func (h *Handler) ListMyBooks(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit, page := parsePage(r)
	var favorite *bool
	if v := q.Get("favorite"); v == "true" || v == "false" {
		b := v == "true"
		favorite = &b
	}
	f := repository.PersonalFilter{
		Query:          q.Get("q"),
		PhysicalStatus: q.Get("physical_status"),
		DigitalStatus:  q.Get("digital_status"),
		ReadingStatus:  q.Get("reading_status"),
		Favorite:       favorite,
	}
	books, total, err := h.Personal.ListUserBooks(r.Context(), userID, f, repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list books.")
		return
	}
	if books == nil {
		books = []collection.UserBook{}
	}
	writeJSON(w, http.StatusOK, books, map[string]int{"limit": limit, "page": page, "total": total})
}

// POST /api/v1/me/books — authenticated.
func (h *Handler) AddMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		BookID         string `json:"book_id"`
		PhysicalStatus string `json:"physical_status"`
		DigitalStatus  string `json:"digital_status"`
		ReadingStatus  string `json:"reading_status"`
		IsFavorite     bool   `json:"is_favorite"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	ub, err := h.PersonalSvc.AddToLibrary(r.Context(), userID, body.BookID, body.PhysicalStatus, body.DigitalStatus, body.ReadingStatus, body.IsFavorite)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ub, nil)
}

// GET /api/v1/me/books/{id} — authenticated (owner only).
func (h *Handler) GetMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ub, nil)
}

// PATCH /api/v1/me/books/{id} — authenticated (owner only).
func (h *Handler) UpdateMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		PhysicalStatus *string `json:"physical_status"`
		DigitalStatus  *string `json:"digital_status"`
		ReadingStatus  *string `json:"reading_status"`
		IsFavorite     *bool   `json:"is_favorite"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	existing, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	ub := *existing
	if body.PhysicalStatus != nil {
		ub.PhysicalStatus = *body.PhysicalStatus
	}
	if body.DigitalStatus != nil {
		ub.DigitalStatus = *body.DigitalStatus
	}
	if body.ReadingStatus != nil {
		ub.ReadingStatus = *body.ReadingStatus
	}
	if body.IsFavorite != nil {
		ub.IsFavorite = *body.IsFavorite
	}
	updated, err := h.PersonalSvc.UpdateStatuses(r.Context(), userID, &ub)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/me/books/{id} — authenticated (owner only; removes entry).
func (h *Handler) DeleteMyBook(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	if err := h.Personal.DeleteUserBook(r.Context(), userID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/me/books/{id}/progress — authenticated.
func (h *Handler) GetProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	p, err := h.Personal.GetProgress(r.Context(), ub.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p, nil)
}

// PUT /api/v1/me/books/{id}/progress — authenticated.
func (h *Handler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		CurrentPage int `json:"current_page"`
		TotalPages  int `json:"total_pages"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	p, err := h.PersonalSvc.UpdateProgress(r.Context(), userID, r.PathValue("id"), body.CurrentPage, body.TotalPages)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p, nil)
}

// GET /api/v1/me/books/{id}/sessions — authenticated.
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	sessions, err := h.Personal.ListSessions(r.Context(), ub.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list sessions.")
		return
	}
	if sessions == nil {
		sessions = []reading.Session{}
	}
	writeJSON(w, http.StatusOK, sessions, nil)
}

// POST /api/v1/me/books/{id}/sessions — authenticated.
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var s reading.Session
	if err := decodeJSON(r, &s); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.PersonalSvc.AddSession(r.Context(), userID, r.PathValue("id"), &s)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// GET /api/v1/me/books/{id}/notes — authenticated (private).
func (h *Handler) ListNotes(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	notes, err := h.Personal.ListNotes(r.Context(), ub.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list notes.")
		return
	}
	if notes == nil {
		notes = []review.Note{}
	}
	writeJSON(w, http.StatusOK, notes, nil)
}

// POST /api/v1/me/books/{id}/notes — authenticated (private).
func (h *Handler) CreateNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var n review.Note
	if err := decodeJSON(r, &n); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.PersonalSvc.CreateNote(r.Context(), userID, r.PathValue("id"), &n)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// PATCH /api/v1/me/books/{id}/notes/{noteId} — authenticated.
func (h *Handler) UpdateNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		PageNumber *int   `json:"page_number"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	updated, err := h.PersonalSvc.UpdateNote(r.Context(), userID, r.PathValue("id"), r.PathValue("noteId"), body.Title, body.Content, body.PageNumber)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/me/books/{id}/notes/{noteId} — authenticated.
func (h *Handler) DeleteNote(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if err := h.Personal.DeleteNote(r.Context(), ub.ID, r.PathValue("noteId")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/me/books/{id}/review — authenticated.
func (h *Handler) GetReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	rv, err := h.Personal.GetReview(r.Context(), ub.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rv, nil)
}

// PUT /api/v1/me/books/{id}/review — authenticated.
func (h *Handler) UpsertReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		Rating     *int   `json:"rating"`
		Reaction   string `json:"reaction"`
		ReviewText string `json:"review_text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	rv, err := h.PersonalSvc.UpsertReview(r.Context(), userID, r.PathValue("id"), body.Rating, body.Reaction, body.ReviewText)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rv, nil)
}

// DELETE /api/v1/me/books/{id}/review — authenticated.
func (h *Handler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	ub, err := h.Personal.GetUserBook(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if err := h.Personal.DeleteReview(r.Context(), ub.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to delete review.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
