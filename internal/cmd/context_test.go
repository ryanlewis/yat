package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
)

func TestNewRunContext_Valid(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "tk-001.md", "---\nid: TK-001\ntitle: \"Task\"\nstatus: todo\n---\n")
	writeItemFile(t, dir, "tk-002.md", "---\nid: TK-002\ntitle: \"Task 2\"\nstatus: todo\ndependencies: [TK-001]\n---\n")

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 2 {
		t.Errorf("Items = %d, want 2", len(rc.Items))
	}
	if rc.Graph == nil {
		t.Error("Graph is nil")
	}
}

func TestNewRunContext_DuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: TK-001\ntitle: \"First\"\nstatus: todo\n---\n")
	writeItemFile(t, dir, "b.md", "---\nid: TK-001\ntitle: \"Duplicate\"\nstatus: todo\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestNewRunContext_UnknownDependencySaysFilesWereSkipped(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: TK-001\nstatus: todo\ndependencies: [TK-002]\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil || err.Error() != `item TK-001 has unknown dependency "TK-002"` {
		t.Fatalf("with nothing skipped, err = %v", err)
	}

	if err = os.Symlink("nowhere.md", filepath.Join(dir, "TK-002.md")); err != nil {
		t.Fatal(err)
	}

	_, err = NewRunContext(dir, false, defaultConfig())

	want := `item TK-001 has unknown dependency "TK-002" (1 file was skipped as unreadable; see warnings)`
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %s", err, want)
	}
}

func TestNewRunContext_CyclicDeps(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: TK-001\ntitle: \"A\"\nstatus: todo\ndependencies: [TK-002]\n---\n")
	writeItemFile(t, dir, "b.md", "---\nid: TK-002\ntitle: \"B\"\nstatus: todo\ndependencies: [TK-001]\n---\n")

	_, err := NewRunContext(dir, false, defaultConfig())
	if err == nil {
		t.Fatal("expected error for cyclic dependencies")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected 'cycle' in error, got: %v", err)
	}
}

func TestNewRunContext_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	rc, err := NewRunContext(dir, false, defaultConfig())
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 0 {
		t.Errorf("Items = %d, want 0", len(rc.Items))
	}
}

func TestNewRunContext_CustomStatuses(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "a.md", "---\nid: A\nstate: backlog\n---\n")

	cfg := &config.Config{
		Statuses: config.StatusGroups{
			Done:    []string{"done", "shipped"},
			Active:  []string{"in-progress", "review"},
			Initial: []string{"backlog"},
		},
		FieldAliases: map[string]string{"status": "state"},
	}

	rc, err := NewRunContext(dir, false, cfg)
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}
	if len(rc.Items) != 1 {
		t.Fatalf("Items = %d, want 1", len(rc.Items))
	}
	if rc.Items[0].Status != "backlog" {
		t.Errorf("Status = %q, want backlog", rc.Items[0].Status)
	}
	if rc.StatusField != "state" {
		t.Errorf("StatusField = %q, want state", rc.StatusField)
	}
}
