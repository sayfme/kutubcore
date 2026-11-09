package handler

import (
	"net/http"

	"kutubcore/internal/middleware"
)

// GET /api/v1/users/me — authenticated (owner view).
func (h *Handler) MeProfile(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	writeJSON(w, http.StatusOK, u, nil)
}

// GET /api/v1/users/me/roles — authenticated.
func (h *Handler) MeRoles(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": u.Roles}, nil)
}

// POST /api/v1/users/me/roles/organizer — authenticated, self-activation (Blueprint §4).
func (h *Handler) ActivateOrganizer(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	roles, err := h.Auth.ActivateOrganizer(r.Context(), u.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Refresh user roles in the response.
	u.Roles = roles
	writeJSON(w, http.StatusOK, u, nil)
}
