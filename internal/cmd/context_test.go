package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
	"github.com/ryanlewis/yat/internal/item"
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

// Starts, completes and adds of the same items, all loaded before any runs, as
// when several yat processes start at once. Each holds the items directory's
// lock around its read, check and write, so a start never undoes a complete
// and adds still take distinct ids.
func TestLockAndReload_ConcurrentStartCompleteAdd(t *testing.T) {
	dir := addTestDir(t)

	const rounds = 4

	cmds := make([]interface{ Run(*RunContext) error }, 0, 5*rounds)

	for i := range rounds {
		cmds = append(cmds,
			&StartCmd{ID: "TK-002"},
			&CompleteCmd{ID: "TK-002"},
			&StartCmd{ID: "ST-01"},
			&CompleteCmd{ID: "ST-01"},
			&AddCmd{Title: fmt.Sprintf("Task %d", i), Type: "Task", stdin: strings.NewReader("")},
		)
	}

	rcs := make([]*RunContext, len(cmds))
	for i := range rcs {
		rcs[i], _ = newAddContext(t, dir, defaultConfig(), false)
	}

	errs := make(chan error, len(cmds))

	for i, c := range cmds {
		go func() { errs <- c.Run(rcs[i]) }()
	}

	for range cmds {
		if err := <-errs; err != nil && !strings.Contains(err.Error(), "already done") {
			t.Errorf("Run: %v", err)
		}
	}

	items, _, err := item.LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}

	byID, err := item.Index(items)
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 3+rounds {
		t.Errorf("got %d items, want %d", len(items), 3+rounds)
	}

	for _, id := range []string{"TK-002", "ST-01"} {
		if got := byID[id].Status; got != item.StatusDone {
			t.Errorf("%s status = %s, want done (%s)", id, got, filepath.Base(byID[id].FilePath))
		}
	}
}

// lockingWriter takes the items directory's lock on every write, to show the
// command writing to it no longer holds it.
type lockingWriter struct {
	dir string
	err error
}

func (w *lockingWriter) Write(p []byte) (int, error) {
	unlock, err := item.LockDir(w.dir)
	if err != nil {
		w.err = err

		return len(p), nil
	}

	unlock()

	return len(p), nil
}

// add, start and complete release the lock before printing, so a reader slow
// to drain stdout does not hold up other yat commands.
func TestLockAndReload_ReleasedBeforeOutput(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		dir := addTestDir(t)

		for _, c := range []interface{ Run(*RunContext) error }{
			&AddCmd{Title: "New", Type: "Task", stdin: strings.NewReader("")},
			&StartCmd{ID: "TK-002"},
			&StartCmd{ID: "TK-002"}, // already in progress
			&CompleteCmd{ID: "TK-002"},
		} {
			rc, _ := newAddContext(t, dir, defaultConfig(), jsonMode)
			w := &lockingWriter{dir: dir}
			rc.Stdout = w

			if err := c.Run(rc); err != nil {
				t.Fatalf("%T: %v", c, err)
			}

			if w.err != nil {
				t.Errorf("%T (json %v) printed while holding the lock: %v", c, jsonMode, w.err)
			}
		}
	}
}
