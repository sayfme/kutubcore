// Command kutubcore runs the KutubCore service.
// Blueprint §2 standalone mode: Go + SQLite on one computer;
// browser/PWA accesses the local service over HTTP.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kutubcore/internal/app"
	"kutubcore/internal/backup"
	"kutubcore/internal/config"
	"kutubcore/internal/database"
	"kutubcore/internal/database/sqlite"
	"kutubcore/migrations"
)

func main() {
	// Subcommands (Blueprint §15): backup / restore. No args runs the server.
	if len(os.Args) > 1 && (os.Args[1] == "backup" || os.Args[1] == "restore") {
		if err := runBackupCmd(os.Args[1]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	cfg := config.Load()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// runBackupCmd implements:
//
//	kutubcore backup [--db PATH] [--files DIR] [--dest DIR]
//	kutubcore restore <backupDir> [--db PATH] [--files DIR]
//
// Restore requires the server to be stopped (see docs).
func runBackupCmd(cmd string) error {
	cfg := config.Load()
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	dbPath := fs.String("db", cfg.DBPath, "SQLite database path")
	filesDir := fs.String("files", cfg.FileDir, "uploaded-files directory")
	dest := fs.String("dest", "data/backups", "backup root directory")

	if cmd == "backup" {
		if err := fs.Parse(os.Args[2:]); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		db, err := sqlite.Open(ctx, *dbPath, logger)
		if err != nil {
			return err
		}
		defer db.Close()
		dir, err := backup.Create(ctx, db, *filesDir, *dest)
		if err != nil {
			return err
		}
		fmt.Println("backup created:", dir)
		fmt.Println("manifest:", dir+"/manifest.json")
		return nil
	}

	// restore: Go's flag package stops at the first positional arg, so
	// flags must be extracted manually to allow any order:
	//   kutubcore restore <backupDir> [--db PATH] [--files DIR]
	//   kutubcore restore [--db PATH] <backupDir>
	remaining, rDb, rFiles := parseRestoreFlags(os.Args[2:], cfg.DBPath, cfg.FileDir)
	if len(remaining) < 1 {
		fmt.Fprintln(os.Stderr, "usage: kutubcore restore <backupDir> [--db PATH] [--files DIR]  (server must be stopped)")
		return fmt.Errorf("missing <backupDir>")
	}
	dir := remaining[0]
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	m, err := backup.Restore(ctx, dir, rDb, rFiles)
	if err != nil {
		return err
	}
	fmt.Printf("restored backup from %s (created %s, schema %s)\n", dir, m.CreatedAt, m.SchemaVersion)
	fmt.Println("start the server to apply migrations and verify:", rDb)
	return nil
}

// lanIP returns the first non-loopback private IPv4 address, or "".
func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		var ip net.IP
		if ipnet, ok := a.(*net.IPNet); ok {
			ip = ipnet.IP
		} else {
			parsed := net.ParseIP(a.String())
			if parsed == nil {
				continue
			}
			ip = parsed
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.To4() == nil {
			continue
		}
		return ip.String()
	}
	return ""
}

func isLocalhostBind(addr string) bool {
	return startsWith(addr, "localhost:") || startsWith(addr, "127.0.0.1:") || startsWith(addr, "[::1]:")
}

func portOf(addr string) string {
	if i := len(addr) - 1; i >= 0 {
		for i >= 0 && addr[i] != ':' {
			i--
		}
		if i >= 0 && i+1 < len(addr) {
			return addr[i+1:]
		}
	}
	return "8080"
}

func startsWith(s, prefix string) bool { return len(s) >= len(prefix) && s[:len(prefix)] == prefix }

// parseRestoreFlags splits args into positional args and --db/--files values,
// accepting flags in any position (flag.FlagSet alone stops at positionals).
func parseRestoreFlags(args []string, dbDef, filesDef string) (positional []string, dbPath, filesDir string) {
	dbPath, filesDir = dbDef, filesDef
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--db" && i+1 < len(args):
			dbPath = args[i+1]
			i++
		case a == "--files" && i+1 < len(args):
			filesDir = args[i+1]
			i++
		case len(a) > 5 && a[:5] == "--db=":
			dbPath = a[5:]
		case len(a) > 8 && a[:8] == "--files=":
			filesDir = a[8:]
		case a == "--":
			positional = append(positional, args[i+1:]...)
			return positional, dbPath, filesDir
		default:
			positional = append(positional, a)
		}
	}
	return positional, dbPath, filesDir
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sqlite.Open(ctx, cfg.DBPath, logger)
	if err != nil {
		return err
	}
	defer db.Close()

	applied, err := database.Run(ctx, db, migrations.SQLite, "sqlite")
	if err != nil {
		return err
	}
	if len(applied) > 0 {
		logger.Info("migrations applied", "count", len(applied), "migrations", applied)
	}

	application := app.New(db, logger, cfg.SessionTTL, cfg.CookieSecure, cfg.FileDir)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout(),
		ReadTimeout:       cfg.ReadTimeout(),
		WriteTimeout:      cfg.WriteTimeout(),
		IdleTimeout:       cfg.IdleTimeout(),
	}

	useTLS := cfg.TLSCertFile != "" && cfg.TLSKeyFile != ""
	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "db", cfg.DBPath, "tls", useTLS)
		// ":PORT" and "0.0.0.0:*" also bind the LAN, not just localhost.
		if !isLocalhostBind(cfg.Addr) {
			if ip := lanIP(); ip != "" {
				scheme := "http"
				if useTLS {
					scheme = "https"
				}
				logger.Info("lan_url", "url", scheme+"://"+ip+":"+portOf(cfg.Addr), "note", "open this on any device on the same network")
			}
		}
		if !isLocalhostBind(cfg.Addr) && !cfg.CookieSecure && !useTLS {
			logger.Warn("serving on LAN over plain HTTP with COOKIE_SECURE=false — session cookies travel in cleartext on this network (LAN only, trusted network)")
		}
		var err error
		if useTLS {
			err = srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
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
