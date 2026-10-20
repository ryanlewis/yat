package item

import (
	"os"
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

func TestLockDir_ReleasesAndLeavesDirAsItWas(t *testing.T) {
	dir := t.TempDir()

	unlock, err := LockDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	unlock()

	// A second lock succeeds at once after release.
	unlock, err = LockDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	unlock()

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("unexpected files left in dir: %v", entries)
	}
}
