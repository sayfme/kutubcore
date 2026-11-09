package handler

import (
	"net/http"

	"kutubcore/internal/middleware"
)

// POST /api/v1/auth/register — public.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Mobile      string `json:"mobile"`
		Password    string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	u, err := h.Auth.Register(r.Context(), body.DisplayName, body.Email, body.Mobile, body.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Auto-login after register.
	token, _, err := h.Auth.Login(r.Context(), firstNonEmpty(body.Email, body.Mobile), body.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		// Registration succeeded; login failure is not fatal to the 201.
		writeJSON(w, http.StatusCreated, u, nil)
		return
	}
	middleware.SetSessionCookie(w, token, h.sessionMaxAge(), h.CookieSecure)
	writeJSON(w, http.StatusCreated, u, nil)
}

// POST /api/v1/auth/login — public.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Mobile     string `json:"mobile"`
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	identifier := firstNonEmpty(body.Identifier, body.Email, body.Mobile)
	token, u, err := h.Auth.Login(r.Context(), identifier, body.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	middleware.SetSessionCookie(w, token, h.sessionMaxAge(), h.CookieSecure)
	writeJSON(w, http.StatusOK, u, nil)
}

// POST /api/v1/auth/logout — authenticated (also fine for guests).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("kutubcore_session"); err == nil && c.Value != "" {
		_ = h.Auth.Logout(r.Context(), c.Value)
	}
	middleware.ClearSessionCookie(w, h.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/auth/me — authenticated.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	writeJSON(w, http.StatusOK, u, nil)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
