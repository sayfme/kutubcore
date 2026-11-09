package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("COOKIE_SECURE", "")
	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.DBPath != "data/kutubcore.db" {
		t.Fatalf("DBPath = %q, want data/kutubcore.db", cfg.DBPath)
	}
	if cfg.CookieSecure {
		t.Fatal("CookieSecure should default false (localhost http)")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("COOKIE_SECURE", "true")
	cfg := Load()
	if cfg.Addr != ":9090" || cfg.DBPath != "/tmp/test.db" {
		t.Fatalf("got %+v", cfg)
	}
	if !cfg.CookieSecure {
		t.Fatal("CookieSecure should be true when COOKIE_SECURE=true")
	}
}
