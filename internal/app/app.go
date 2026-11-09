// Package app wires KutubCore's dependencies together.
// Kept separate from cmd/ so tests can build the app without a main.
package app

import (
	"database/sql"
	"log/slog"
	"net/http"

	"kutubcore/internal/handler"
	"kutubcore/internal/middleware"
)

// App is the assembled application.
type App struct {
	Handler http.Handler
	DB      *sql.DB
	Logger  *slog.Logger
}

// New builds the app with the given DB and logger. No routes are
// registered here beyond what handler.Routes defines.
func New(db *sql.DB, logger *slog.Logger) *App {
	h := &handler.Handler{DB: db, Logger: logger}
	mux := middleware.RequestID(logger, h.Routes())
	return &App{Handler: mux, DB: db, Logger: logger}
}
