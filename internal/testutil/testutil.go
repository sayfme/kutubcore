// Package testutil builds an in-memory test app for handler tests.
package testutil

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"kutubcore/internal/app"
	"kutubcore/internal/database"
	"kutubcore/internal/database/sqlite"
	"kutubcore/internal/service"
	"kutubcore/migrations"

	_ "modernc.org/sqlite"
)

// NewDB creates an in-memory SQLite DB with all migrations applied.
func NewDB(t interface {
	Helper()
	Fatal(...any)
}) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(context.Background(), `PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	// Same pragmas as production (single writer, FKs).
	if _, err := database.Run(context.Background(), db, migrations.SQLite, "sqlite"); err != nil {
		t.Fatal(err)
	}
	return db
}

// NewApp builds the full handler stack on db (uploads go to a temp dir).
func NewApp(db *sql.DB) *app.App {
	logger := slog.New(slog.NewTextHandler(noopWriter{}, nil))
	userRepo := sqlite.NewUserRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	catalogRepo := sqlite.NewCatalogRepo(db)
	personalRepo := sqlite.NewPersonalRepo(db)
	acquisitionRepo := sqlite.NewAcquisitionRepo(db)
	fileDir, err := os.MkdirTemp("", "kutubcore-test-*")
	if err != nil {
		panic(err)
	}
	tokenRepo := sqlite.NewTokenRepo(db)
	d := app.Deps{
		Auth:           &service.AuthService{Users: userRepo, Sessions: sessionRepo, TTL: 24 * time.Hour},
		Catalog:        &service.CatalogService{Books: catalogRepo, Writers: catalogRepo, Audit: catalogRepo},
		Account:        &service.AccountService{Users: userRepo, Tokens: tokenRepo, Sessions: sessionRepo},
		Personal:       personalRepo,
		PersonalSvc:    &service.PersonalService{Personal: personalRepo, Books: catalogRepo, Acquisitions: acquisitionRepo, Files: catalogRepo},
		Dirs:           catalogRepo,
		DirSvc:         &service.DirectoryService{Dirs: catalogRepo, Audit: catalogRepo},
		Acquisitions:   acquisitionRepo,
		AcquisitionSvc: &service.AcquisitionService{Acquisitions: acquisitionRepo, Books: catalogRepo},
		Files:          catalogRepo,
		FileSvc:        &service.FileService{Files: catalogRepo, Books: catalogRepo, Audit: catalogRepo, FileDir: fileDir},
		FileDir:        fileDir,
		Books:          catalogRepo,
		Writers:        catalogRepo,
		Ref:            catalogRepo,
		SessionTTL:     24 * time.Hour,
	}
	return app.NewWithDeps(db, logger, d)
}

// Noop avoids log spam in tests.
type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }
