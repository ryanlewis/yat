//go:build !unix

package item

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLockDir_RemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, lockName)

	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	stale := time.Now().Add(-2 * lockStale)
	if err := os.Chtimes(path, stale, stale); err != nil {
		t.Fatal(err)
	}

	unlock, err := LockDir(dir)
	if err != nil {
		t.Fatalf("expected the stale lock to be taken over, got: %v", err)
	}

	unlock()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("lock file left after unlock: %v", err)
	}
}

// A folder the user cannot create the lock file in is used without a lock, with
// one warning.
func TestLockDir_PermissionDeniedGoesOnWithoutLock(t *testing.T) {
	dir := t.TempDir()

	old := openLock
	openLock = func(string) (*os.File, error) { return nil, fs.ErrPermission }
	t.Cleanup(func() { openLock = old })

	var buf bytes.Buffer

	oldOut := warnOut
	warnOut = &buf
	t.Cleanup(func() { warnOut = oldOut })

	unlock, err := LockDir(dir)
	if err != nil {
		t.Fatalf("LockDir: %v", err)
	}

	unlock()

	want := "warning: could not lock " + dir + ": permission denied; continuing without a lock\n"
	if buf.String() != want {
		t.Errorf("warning = %q, want %q", buf.String(), want)
	}
}
