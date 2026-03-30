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

func TestMergeFrom(t *testing.T) {
	primary := config.Config{
		Dir:          "/primary",
		FieldAliases: map[string]string{"status": "current"},
	}

	secondary := config.Config{
		Dir:      "/secondary",
		ReadOnly: true,
		Statuses: config.StatusGroups{
			Done:    []string{"shipped"},
			Active:  []string{"wip"},
			Initial: []string{"new"},
		},
		Phases:       []string{"alpha", "beta"},
		FieldAliases: map[string]string{"status": "condition", "priority": "urgency"},
	}

	primary.MergeFrom(&secondary)

	if primary.Dir != "/primary" {
		t.Errorf("Dir: expected /primary, got %s", primary.Dir)
	}

	if !primary.ReadOnly {
		t.Error("ReadOnly: expected true via OR")
	}

	if !primary.Statuses.IsDone("shipped") {
		t.Error("Statuses: expected secondary statuses")
	}

	if len(primary.Phases) != 2 || primary.Phases[0] != "alpha" {
		t.Errorf("Phases: expected secondary phases, got %v", primary.Phases)
	}

	if primary.FieldAliases["status"] != "current" {
		t.Errorf("FieldAliases: expected primary status alias, got %q", primary.FieldAliases["status"])
	}

	if primary.FieldAliases["priority"] != "urgency" {
		t.Errorf("FieldAliases: expected secondary priority alias, got %q", primary.FieldAliases["priority"])
	}
}

func TestMergeFrom_NilAliases(t *testing.T) {
	primary := config.Config{}
	secondary := config.Config{
		FieldAliases: map[string]string{"status": "current"},
	}

	primary.MergeFrom(&secondary)

	if primary.FieldAliases["status"] != "current" {
		t.Errorf("expected alias from secondary, got %q", primary.FieldAliases["status"])
	}
}

func TestMergeFrom_EmptySecondary(t *testing.T) {
	primary := config.Config{
		Dir:    "/primary",
		Phases: []string{"v1"},
	}

	empty := config.Config{}
	primary.MergeFrom(&empty)

	if primary.Dir != "/primary" {
		t.Errorf("Dir: expected /primary, got %s", primary.Dir)
	}

	if len(primary.Phases) != 1 {
		t.Errorf("Phases should be unchanged, got %v", primary.Phases)
	}
}

func TestLoadWithFallback_UsesItemsDir(t *testing.T) {
	t.Chdir(t.TempDir()) // cwd has no config

	itemsDir := t.TempDir()
	content := []byte("readonly: true\nfield_aliases:\n  id: localId\n")
	if err := os.WriteFile(filepath.Join(itemsDir, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback(itemsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.ReadOnly {
		t.Error("expected ReadOnly from items-dir config")
	}

	if cfg.FieldAliases["id"] != "localId" {
		t.Errorf("expected alias id→localId, got %q", cfg.FieldAliases["id"])
	}
}

func TestLoadWithFallback_CwdDirTakesPrecedence(t *testing.T) {
	cwdDir := t.TempDir()
	t.Chdir(cwdDir)

	cwdContent := []byte("dir: /from-cwd\n")
	if err := os.WriteFile(filepath.Join(cwdDir, ".yat.yaml"), cwdContent, 0o644); err != nil {
		t.Fatal(err)
	}

	itemsDir := t.TempDir()
	itemsContent := []byte("dir: /from-items\n")
	if err := os.WriteFile(filepath.Join(itemsDir, ".yat.yaml"), itemsContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback(itemsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != "/from-cwd" {
		t.Errorf("expected cwd dir to win, got %q", cfg.Dir)
	}
}

func TestLoadWithFallback_MergesConfigs(t *testing.T) {
	cwdDir := t.TempDir()
	t.Chdir(cwdDir)

	cwdContent := []byte("dir: /some/items\nfield_aliases:\n  status: state\n")
	if err := os.WriteFile(filepath.Join(cwdDir, ".yat.yaml"), cwdContent, 0o644); err != nil {
		t.Fatal(err)
	}

	itemsDir := t.TempDir()
	itemsContent := []byte("readonly: true\nphases:\n  - planning\n  - building\nfield_aliases:\n  status: condition\n  priority: urgency\n")
	if err := os.WriteFile(filepath.Join(itemsDir, ".yat.yaml"), itemsContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback(itemsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dir from CWD config.
	if cfg.Dir != "/some/items" {
		t.Errorf("expected dir from cwd, got %q", cfg.Dir)
	}

	// ReadOnly OR: items-dir sets true.
	if !cfg.ReadOnly {
		t.Error("expected readonly=true via OR semantics")
	}

	// Phases from items-dir (CWD has none).
	if len(cfg.Phases) != 2 || cfg.Phases[0] != "planning" {
		t.Errorf("expected phases from items-dir, got %v", cfg.Phases)
	}

	// FieldAliases: CWD's status→state wins, items-dir's priority→urgency fills gap.
	if cfg.FieldAliases["status"] != "state" {
		t.Errorf("expected cwd alias for status, got %q", cfg.FieldAliases["status"])
	}

	if cfg.FieldAliases["priority"] != "urgency" {
		t.Errorf("expected items-dir alias for priority, got %q", cfg.FieldAliases["priority"])
	}
}

func TestLoadWithFallback_SecondaryFromPrimaryDir(t *testing.T) {
	// CWD config points dir at a subdirectory; config exists in its parent.
	// No --dir flag (fallbackDir=""), so secondary search uses primary.Dir.
	root := t.TempDir()
	itemsDir := filepath.Join(root, "spec")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Config near items: custom statuses.
	rootContent := []byte("statuses:\n  done: [shipped]\n  active: [wip]\n  initial: [new]\n")
	if err := os.WriteFile(filepath.Join(root, ".yat.yaml"), rootContent, 0o644); err != nil {
		t.Fatal(err)
	}

	// CWD config: just dir.
	cwdDir := t.TempDir()
	t.Chdir(cwdDir)

	cwdContent := []byte("dir: " + itemsDir + "\n")
	if err := os.WriteFile(filepath.Join(cwdDir, ".yat.yaml"), cwdContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Dir != itemsDir {
		t.Errorf("expected dir=%s, got %s", itemsDir, cfg.Dir)
	}

	if !cfg.Statuses.IsDone("shipped") {
		t.Error("expected statuses from items-dir config to be merged")
	}
}

func TestLoadWithFallback_ReadOnlyOR(t *testing.T) {
	cwdDir := t.TempDir()
	t.Chdir(cwdDir)

	// CWD config omits readonly (defaults to false).
	cwdContent := []byte("dir: /items\n")
	if err := os.WriteFile(filepath.Join(cwdDir, ".yat.yaml"), cwdContent, 0o644); err != nil {
		t.Fatal(err)
	}

	itemsDir := t.TempDir()
	itemsContent := []byte("readonly: true\n")
	if err := os.WriteFile(filepath.Join(itemsDir, ".yat.yaml"), itemsContent, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback(itemsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.ReadOnly {
		t.Error("expected readonly=true when items-dir config sets it")
	}
}

func TestLoadWithFallback_EmptyFallback(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := config.LoadWithFallback("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get defaults when no config found anywhere.
	if cfg.Statuses.IsEmpty() {
		t.Error("expected default statuses to be populated")
	}
}

func TestLoadWithFallback_AncestorOfItemsDir(t *testing.T) {
	t.Chdir(t.TempDir()) // cwd has no config

	root := t.TempDir()
	content := []byte("readonly: true\n")
	if err := os.WriteFile(filepath.Join(root, ".yat.yaml"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	itemsDir := filepath.Join(root, "project", "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadWithFallback(itemsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.ReadOnly {
		t.Error("expected config found in ancestor of items dir")
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
