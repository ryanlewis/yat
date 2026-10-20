//go:build unix

package item

import (
	"bytes"
	"syscall"
	"testing"
)

// A filesystem that cannot lock is used without a lock, with one warning.
func TestLockDir_UnsupportedGoesOnWithoutLock(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.ENOLCK, syscall.EROFS} {
		t.Run(errno.Error(), func(t *testing.T) {
			dir := t.TempDir()

			old := flock
			flock = func(int, int) error { return errno }
			t.Cleanup(func() { flock = old })

			var buf bytes.Buffer

			oldOut := warnOut
			warnOut = &buf
			t.Cleanup(func() { warnOut = oldOut })

			unlock, err := LockDir(dir)
			if err != nil {
				t.Fatalf("LockDir: %v", err)
			}

			unlock()

			want := "warning: could not lock " + dir + ": " + errno.Error() + "; continuing without a lock\n"
			if buf.String() != want {
				t.Errorf("warning = %q, want %q", buf.String(), want)
			}
		})
	}
}
