//go:build unix

package item

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// LockDir takes an advisory lock (flock) on dir itself, waiting while another
// process holds it. No file is created, and the kernel drops the lock if the
// process dies. Call the returned function to release it.
func LockDir(dir string) (func(), error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("locking %s: %w", dir, err)
	}

	fd := int(f.Fd()) //nolint:gosec // file descriptors fit in int
	deadline := time.Now().Add(lockWait)

	for {
		err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Closing the descriptor releases the lock.
			return func() { _ = f.Close() }, nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			_ = f.Close()

			return nil, fmt.Errorf("locking %s: %w", dir, err)
		}

		if time.Now().After(deadline) {
			_ = f.Close()

			return nil, fmt.Errorf("timed out waiting for another yat add to finish in %s", dir)
		}

		time.Sleep(lockPoll)
	}
}
