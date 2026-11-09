// Package app wires KutubCore's dependencies together.
// Kept separate from cmd/ so tests can build the app without a main.
package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"kutubcore/internal/database/sqlite"
	"kutubcore/internal/handler"
	"kutubcore/internal/middleware"
	"kutubcore/internal/repository"
	"kutubcore/internal/service"
)

// App is the assembled application.
type App struct {
	Handler http.Handler
	DB      *sql.DB
	Logger  *slog.Logger
}

// Deps lets callers (tests) inject services and repos.
type Deps struct {
	Auth    *service.AuthService
	Catalog *service.CatalogService
	Account *service.AccountService

	Personal    repository.PersonalRepository
	PersonalSvc *service.PersonalService

	Analytics    repository.AnalyticsRepository
	AnalyticsSvc *service.AnalyticsService

	Dirs   repository.DirectoryRepository
	DirSvc *service.DirectoryService

	Acquisitions   repository.AcquisitionRepository
	AcquisitionSvc *service.AcquisitionService

	Files   repository.FileRepository
	FileSvc *service.FileService
	FileDir string

	Books   repository.BookRepository
	Writers repository.WriterRepository
	Ref     repository.ReferenceRepository

	SessionTTL   time.Duration
	CookieSecure bool
}

// New builds the app from a DB and logger using SQLite repos.
// fileDir is the filesystem base for uploads (Blueprint §8); "" -> data/files.
func New(db *sql.DB, logger *slog.Logger, sessionTTL time.Duration, cookieSecure bool, fileDir string) *App {
	if fileDir == "" {
		fileDir = "data/files"
	}
	userRepo := sqlite.NewUserRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	catalogRepo := sqlite.NewCatalogRepo(db)
	personalRepo := sqlite.NewPersonalRepo(db)
	acquisitionRepo := sqlite.NewAcquisitionRepo(db)
	tokenRepo := sqlite.NewTokenRepo(db)
	analyticsRepo := sqlite.NewAnalyticsRepo(db)

	d := Deps{
		Auth: &service.AuthService{
			Users:    userRepo,
			Sessions: sessionRepo,
			TTL:      sessionTTL,
		},
		Catalog: &service.CatalogService{
			Books:   catalogRepo,
			Writers: catalogRepo,
			Audit:   catalogRepo,
		},
		Account: &service.AccountService{
			Users: userRepo, Tokens: tokenRepo, Sessions: sessionRepo,
		},
		Personal: personalRepo,
		PersonalSvc: &service.PersonalService{
			Personal:     personalRepo,
			Books:        catalogRepo,
			Acquisitions: acquisitionRepo,
			Files:        catalogRepo,
		},
		Analytics:    analyticsRepo,
		AnalyticsSvc: &service.AnalyticsService{Repo: analyticsRepo},
		Dirs:         catalogRepo,
		DirSvc: &service.DirectoryService{
			Dirs:  catalogRepo,
			Audit: catalogRepo,
		},
		Acquisitions: acquisitionRepo,
		AcquisitionSvc: &service.AcquisitionService{
			Acquisitions: acquisitionRepo,
			Books:        catalogRepo,
		},
		Files: catalogRepo,
		FileSvc: &service.FileService{
			Files:   catalogRepo,
			Books:   catalogRepo,
			Audit:   catalogRepo,
			FileDir: fileDir,
		},
		FileDir:      fileDir,
		Books:        catalogRepo,
		Writers:      catalogRepo,
		Ref:          catalogRepo,
		SessionTTL:   sessionTTL,
		CookieSecure: cookieSecure,
	}
	return NewWithDeps(db, logger, d)
}

// NewWithDeps builds the app from explicit deps (used by tests).
// Middleware order: RequestID -> SessionAuth -> routes. RequireAuth /
// RequireOrganizer run inside route handlers after SessionAuth has
// populated the request context.
func NewWithDeps(db *sql.DB, logger *slog.Logger, d Deps) *App {
	h := &handler.Handler{
		DB:             db,
		Logger:         logger,
		Auth:           d.Auth,
		Catalog:        d.Catalog,
		Account:        d.Account,
		Personal:       d.Personal,
		PersonalSvc:    d.PersonalSvc,
		Analytics:      d.Analytics,
		AnalyticsSvc:   d.AnalyticsSvc,
		Dirs:           d.Dirs,
		DirSvc:         d.DirSvc,
		Acquisitions:   d.Acquisitions,
		AcquisitionSvc: d.AcquisitionSvc,
		Files:          d.Files,
		FileSvc:        d.FileSvc,
		FileDir:        d.FileDir,
		Books:          d.Books,
		Writers:        d.Writers,
		Ref:            d.Ref,
		SessionTTL:     d.SessionTTL,
		CookieSecure:   d.CookieSecure,
	}
	mux := middleware.RequestID(logger, middleware.SecurityHeaders(d.CookieSecure)(middleware.SessionAuth(d.Auth)(h.Routes())))
	return &App{Handler: mux, DB: db, Logger: logger}
}
