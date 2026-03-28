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

func TestLoad_ReadOnly(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("readonly: true\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.ReadOnly {
		t.Fatal("expected ReadOnly to be true")
	}
}

func TestLoad_ReadOnlyDefault(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: /some/path\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ReadOnly {
		t.Fatal("expected ReadOnly to default to false")
	}
}

func TestLoad_Statuses(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte(`statuses:
  done: [done, shipped]
  active: [in-progress, review]
  initial: [backlog]
`)
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Statuses.Done) != 2 || cfg.Statuses.Done[0] != "done" || cfg.Statuses.Done[1] != "shipped" {
		t.Errorf("Done = %v", cfg.Statuses.Done)
	}
	if !cfg.Statuses.IsDone("shipped") {
		t.Error("expected IsDone(shipped) = true")
	}
	if !cfg.Statuses.IsActive("review") {
		t.Error("expected IsActive(review) = true")
	}
	if cfg.Statuses.DefaultInitial() != "backlog" {
		t.Errorf("DefaultInitial = %q, want backlog", cfg.Statuses.DefaultInitial())
	}
}

func TestLoad_FieldAliases(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte(`field_aliases:
  status: state
  dependencies: blocks
`)
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.FieldAliases["status"] != "state" {
		t.Errorf("FieldAliases[status] = %q, want state", cfg.FieldAliases["status"])
	}
}

func TestLoad_StatusesDuplicateGroup(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte(`statuses:
  done: [done, shipped]
  active: [shipped]
  initial: [draft]
`)
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for duplicate status across groups")
	}
}

func TestLoad_InvalidAliasKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte(`field_aliases:
  bogus: state
`)
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid alias key")
	}
}

func TestDefaults_NoStatusesKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	content := []byte("dir: /some/path\n")
	if err := os.WriteFile(filepath.Join(dir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantDone := []string{"done", "complete", "completed", "closed"}
	if len(cfg.Statuses.Done) != len(wantDone) || cfg.Statuses.Done[0] != "done" {
		t.Errorf("expected default Done = %v, got %v", wantDone, cfg.Statuses.Done)
	}
	wantActive := []string{"in-progress", "active", "started"}
	if len(cfg.Statuses.Active) != len(wantActive) || cfg.Statuses.Active[0] != "in-progress" {
		t.Errorf("expected default Active = %v, got %v", wantActive, cfg.Statuses.Active)
	}
	wantInitial := []string{"draft", "todo", "backlog", "new"}
	if len(cfg.Statuses.Initial) != len(wantInitial) || cfg.Statuses.Initial[0] != "draft" {
		t.Errorf("expected default Initial = %v, got %v", wantInitial, cfg.Statuses.Initial)
	}
}

func TestStatusGroups_AllValid(t *testing.T) {
	sg := config.StatusGroups{
		Done:    []string{"done", "shipped"},
		Active:  []string{"in-progress"},
		Initial: []string{"draft"},
	}

	all := sg.AllValid()
	if len(all) != 4 {
		t.Errorf("AllValid returned %d items, want 4", len(all))
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
