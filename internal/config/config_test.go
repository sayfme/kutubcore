package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("DB_PATH", "")
	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.DBPath != "data/kutubcore.db" {
		t.Fatalf("DBPath = %q, want data/kutubcore.db", cfg.DBPath)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("DB_PATH", "/tmp/test.db")
	cfg := Load()
	if cfg.Addr != ":9090" || cfg.DBPath != "/tmp/test.db" {
		t.Fatalf("got %+v", cfg)
	}
}
