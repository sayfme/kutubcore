package handler

import (
	"net/http"
	"strconv"

	"kutubcore/internal/domain/analytics"
	"kutubcore/internal/domain/shelf"
)

// GET /api/v1/me/analytics?months=6 — authenticated.
func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	months, _ := strconv.Atoi(r.URL.Query().Get("months"))
	if months < 1 || months > 24 {
		months = 6
	}
	sum, err := h.AnalyticsSvc.Analytics(r.Context(), userID, months)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to load analytics.")
		return
	}
	writeJSON(w, http.StatusOK, sum, nil)
}

// GET /api/v1/me/goals?month=YYYY-MM — authenticated (defaults to current month).
func (h *Handler) GetGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	g, err := h.AnalyticsSvc.GetGoal(r.Context(), userID, r.URL.Query().Get("month"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g, nil)
}

// PUT /api/v1/me/goals — authenticated {period_month?, books_target}.
func (h *Handler) SetGoal(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var body struct {
		PeriodMonth string `json:"period_month"`
		BooksTarget int    `json:"books_target"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	g, err := h.AnalyticsSvc.SetGoal(r.Context(), userID, body.PeriodMonth, body.BooksTarget)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, g, nil)
}

// GET /api/v1/me/goals/all — authenticated.
func (h *Handler) ListGoals(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	goals, err := h.Analytics.ListGoals(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to list goals.")
		return
	}
	if goals == nil {
		goals = []analytics.Goal{}
	}
	writeJSON(w, http.StatusOK, goals, nil)
}

// GET /api/v1/me/shelf — authenticated (Bookshelf Life stages).
func (h *Handler) Shelf(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	entries, settings, err := h.AnalyticsSvc.ShelfWithStages(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "Failed to load shelf.")
		return
	}
	if entries == nil {
		entries = []shelf.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "settings": settings}, nil)
}

// PUT /api/v1/me/shelf/settings — authenticated {enabled, dust_days, cobweb_days, worm_days}.
func (h *Handler) UpdateShelfSettings(w http.ResponseWriter, r *http.Request) {
	userID, ok := meUser(w, r)
	if !ok {
		return
	}
	var s shelf.Settings
	if err := decodeJSON(r, &s); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid JSON body.")
		return
	}
	if s.DustDays == 0 && s.CobwebDays == 0 && s.WormDays == 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "thresholds are required (dust_days, cobweb_days, worm_days).")
		return
	}
	s.UserID = userID
	updated, err := h.AnalyticsSvc.UpdateShelfSettings(r.Context(), userID, &s)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated, nil)
}
