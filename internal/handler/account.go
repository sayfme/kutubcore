package handler

import (
	"net/http"

	"kutubcore/internal/middleware"
)

// POST /api/v1/auth/forgot-password — public, always 200 (no enumeration).
// Without SMTP the reset link goes to the server log (local/dev). Production
// must wire SMTP/email delivery before exposing this endpoint.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email      string `json:"email"`
		Mobile     string `json:"mobile"`
		Identifier string `json:"identifier"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	identifier := firstNonEmpty(body.Identifier, body.Email, body.Mobile)
	token, err := h.Account.ForgotPassword(r.Context(), identifier)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if token != "" {
		// Delivery is the caller's concern: log it for local/dev (see service).
		msg := "password reset requested for " + identifier + " — open POST /api/v1/auth/reset-password with token="
		if h.Logger != nil {
			// NEVER log the full token at INFO in production; this is the
			// local-dev fallback only (no SMTP configured yet).
			h.Logger.Info("reset_link", "identifier", identifier, "token", token, "hint", msg)
		}
		_ = msg
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true}, nil)
}

// POST /api/v1/auth/reset-password — public.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	if body.Token == "" || body.NewPassword == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "token and new_password are required.")
		return
	}
	if err := h.Account.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reset": true}, nil)
}

// POST /api/v1/auth/verify-email — public (token from the mailed link).
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	if err := h.Account.VerifyEmail(r.Context(), body.Token); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"verified": true}, nil)
}

// POST /api/v1/auth/resend-verification — authenticated.
func (h *Handler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required.")
		return
	}
	token, err := h.Account.CreateVerification(r.Context(), u.ID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if token != "" && h.Logger != nil {
		h.Logger.Info("verification_link", "user_id", u.ID, "token", token)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sent": true}, nil)
}
