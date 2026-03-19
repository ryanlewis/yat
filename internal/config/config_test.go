package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
)

func TestLoad_NoFile(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "" {
		t.Fatalf("expected empty Dir, got %q", cfg.Dir)
	}
}

func TestLoad_WithDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: /absolute/path/to/spec\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/absolute/path/to/spec" {
		t.Fatalf("expected /absolute/path/to/spec, got %q", cfg.Dir)
	}
}

func TestLoad_TildeExpansion(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: ~/notes/project/spec\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cfg, loadErr := config.Load()
	if loadErr != nil {
		t.Fatalf("unexpected error: %v", loadErr)
	}

	want := filepath.Join(home, "notes", "project", "spec")
	if cfg.Dir != want {
		t.Fatalf("expected %q, got %q", want, cfg.Dir)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: [invalid\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
