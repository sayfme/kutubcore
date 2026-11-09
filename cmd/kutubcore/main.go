// Command kutubcore runs the KutubCore service.
// Blueprint §2 standalone mode: Go + SQLite on one computer;
// browser/PWA accesses the local service over HTTP.
package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kutubcore/internal/app"
	"kutubcore/internal/config"
	"kutubcore/internal/database"
	"kutubcore/internal/database/sqlite"
)

//go:embed ../../migrations/sqlite/*.sql
var sqliteMigrations embed.FS

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sqlite.Open(ctx, cfg.DBPath, logger)
	if err != nil {
		return err
	}
	defer db.Close()

	applied, err := database.Run(ctx, db, sqliteMigrations, "migrations/sqlite")
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		logger.Info("migrations applied", "count", len(applied), "migrations", applied)
	}

	application := app.New(db, logger)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout(),
		ReadTimeout:       cfg.ReadTimeout(),
		WriteTimeout:      cfg.WriteTimeout(),
		IdleTimeout:       cfg.IdleTimeout(),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "db", cfg.DBPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Info("shutting down (graceful)")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
