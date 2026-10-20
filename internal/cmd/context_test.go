package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

type runner interface{ Run(*RunContext) error }

// runLocked runs c with a context from NewLockedRunContext, as yat does for a
// command that writes, and releases the lock after.
func runLocked(dir string, c runner) error {
	rc, err := NewLockedRunContext(dir, false, defaultConfig())
	if err != nil {
		return err
	}
	defer rc.Unlock()

	rc.Stdout = io.Discard

	return c.Run(rc)
}

// runWhileLocked holds the items directory's lock while it starts c with a
// locked context and calls change, then releases it and returns c's error. The
// context waits for the lock before it loads, so c sees what change wrote.
func runWhileLocked(t *testing.T, dir string, change func(), c runner) error {
	t.Helper()

	unlock, err := item.LockDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	errs := make(chan error, 1)

	go func() { errs <- runLocked(dir, c) }()

	select {
	case err := <-errs:
		unlock()
		t.Fatalf("%T ran while another held the lock: %v", c, err)
	case <-time.After(50 * time.Millisecond):
	}

	change()
	unlock()

	return <-errs
}

// Starts, completes and adds of the same items, all run at once, as when
// several yat processes start together. Each loads under the items directory's
// lock and holds it until it has written, so a start never undoes a complete
// and adds still take distinct ids.
func TestNewLockedRunContext_ConcurrentStartCompleteAdd(t *testing.T) {
	dir := addTestDir(t)

	const rounds = 4

	cmds := make([]runner, 0, 5*rounds)

	for i := range rounds {
		cmds = append(cmds,
			&StartCmd{ID: "TK-002"},
			&CompleteCmd{ID: "TK-002"},
			&StartCmd{ID: "ST-01"},
			&CompleteCmd{ID: "ST-01"},
			&AddCmd{Title: fmt.Sprintf("Task %d", i), Type: "Task", stdin: strings.NewReader("")},
		)
	}

	errs := make(chan error, len(cmds))

	for _, c := range cmds {
		go func() { errs <- runLocked(dir, c) }()
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

// A file broken while another command holds the lock is skipped, with its
// warning, by the one load a locked context makes.
func TestNewLockedRunContext_LoadsFileBrokenWhileWaiting(t *testing.T) {
	dir := addTestDir(t)
	broken := filepath.Join(dir, "tasks", "TK-009.md")

	var rc *RunContext

	err := runWhileLocked(t, dir, func() {
		if err := os.Symlink("nowhere.md", broken); err != nil {
			t.Fatal(err)
		}
	}, runFunc(func(got *RunContext) error {
		rc = got

		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	if want := []string{broken}; !slices.Equal(rc.skipped, want) {
		t.Errorf("skipped = %q, want %q", rc.skipped, want)
	}
}

// runFunc runs a function as a command.
type runFunc func(*RunContext) error

func (f runFunc) Run(rc *RunContext) error { return f(rc) }

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
func TestNewLockedRunContext_ReleasedBeforeOutput(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		dir := addTestDir(t)

		for _, c := range []runner{
			&AddCmd{Title: "New", Type: "Task", stdin: strings.NewReader("")},
			&StartCmd{ID: "TK-002"},
			&StartCmd{ID: "TK-002"}, // already in progress
			&CompleteCmd{ID: "TK-002"},
		} {
			rc, err := NewLockedRunContext(dir, jsonMode, defaultConfig())
			if err != nil {
				t.Fatal(err)
			}

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
