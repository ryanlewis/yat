package item

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockDir_WaitsThenTimesOut(t *testing.T) {
	dir := t.TempDir()

	unlock, err := LockDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	old := lockWait
	lockWait = 50 * time.Millisecond
	t.Cleanup(func() { lockWait = old })

	if _, err := LockDir(dir); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected a time-out while the lock is held, got: %v", err)
	}
}

func TestLockDir_RemovesStaleLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LockName)

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
