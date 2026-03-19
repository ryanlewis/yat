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

func TestLoad_YmlExtension(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: /path/from/yml\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/path/from/yml" {
		t.Fatalf("expected /path/from/yml, got %q", cfg.Dir)
	}
}

func TestLoad_YamlTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), []byte("dir: /from-yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".yat.yml"), []byte("dir: /from-yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/from-yaml" {
		t.Fatalf("expected /from-yaml to take precedence, got %q", cfg.Dir)
	}
}

func TestLoad_AncestorDirectory(t *testing.T) {
	root := t.TempDir()

	content := []byte("dir: /from-ancestor\n")
	if err := os.WriteFile(filepath.Join(root, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	subdir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(subdir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/from-ancestor" {
		t.Fatalf("expected /from-ancestor, got %q", cfg.Dir)
	}
}

func TestLoad_ClosestConfigWins(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, ".yat.yaml"), []byte("dir: /from-root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(child, ".yat.yaml"), []byte("dir: /from-child\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(child)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/from-child" {
		t.Fatalf("expected /from-child (closest), got %q", cfg.Dir)
	}
}

func TestLoad_LocalOverridesShared(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), []byte("dir: /shared\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, ".yat.local.yaml"), []byte("dir: /local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/local" {
		t.Fatalf("expected /local to override /shared, got %q", cfg.Dir)
	}
}

func TestLoad_LocalYmlExtension(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.WriteFile(filepath.Join(dir, ".yat.local.yml"), []byte("dir: /local-yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/local-yml" {
		t.Fatalf("expected /local-yml, got %q", cfg.Dir)
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
