package handler

import (
	"net/http"

	"kutubcore/internal/domain/book"
	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
)

// GET /api/v1/books — public (guest search/browse, Blueprint §4/§9).
func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, page := parsePage(r)
	f := repository.BookFilter{
		Query:    q.Get("q"),
		Language: q.Get("language"),
		Genre:    q.Get("genre"),
		WriterID: q.Get("writer_id"),
		Sort:     q.Get("sort"),
	}
	books, total, err := h.Books.ListBooks(r.Context(), f, repository.Page{Limit: limit, Page: page})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list books.")
		return
	}
	if books == nil {
		books = []book.Book{}
	}
	writeJSON(w, http.StatusOK, books, map[string]int{"limit": limit, "page": page, "total": total})
}

// GET /api/v1/books/{id} — public.
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := h.Books.GetBook(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b, nil)
}

// POST /api/v1/books — organizer. ?force=true bypasses duplicate warning.
func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var b book.Book
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	force := r.URL.Query().Get("force") == "true"
	created, err := h.Catalog.CreateBook(r.Context(), u.ID, &b, force)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

// PATCH /api/v1/books/{id} — organizer.
func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var b book.Book
	if err := decodeJSON(r, &b); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	b.ID = r.PathValue("id")
	updated, err := h.Catalog.UpdateBook(r.Context(), u.ID, &b)
	if err != nil {
		// Version conflict arrives as validation error; map to 409.
		if vErr, ok := err.(interface{ Error() string }); ok && containsConflict(vErr.Error()) {
			writeError(w, http.StatusConflict, "VERSION_CONFLICT", vErr.Error())
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}

// DELETE /api/v1/books/{id} — organizer (soft delete).
func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	if err := h.Catalog.DeleteBook(r.Context(), u.ID, r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/books/{id}/editions — public.
func (h *Handler) ListEditions(w http.ResponseWriter, r *http.Request) {
	editions, err := h.Books.ListEditions(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list editions.")
		return
	}
	if editions == nil {
		editions = []book.Edition{}
	}
	writeJSON(w, http.StatusOK, editions, nil)
}

// POST /api/v1/books/{id}/editions — organizer.
func (h *Handler) CreateEdition(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	var e book.Edition
	if err := decodeJSON(r, &e); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	created, err := h.Catalog.CreateEdition(r.Context(), u.ID, r.PathValue("id"), &e)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, created, nil)
}

func containsConflict(s string) bool {
	for i := 0; i+8 <= len(s); i++ {
		if s[i:i+8] == "conflict" {
			return true
		}
	}
	return false
}
