//go:build unix

package item

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// flock is syscall.Flock, replaced in tests.
var flock = syscall.Flock

// LockDir takes an advisory lock (flock) on dir itself, waiting while another
// process holds it. No file is created, and the kernel drops the lock if the
// process dies. Call the returned function to release it.
//
// Where the filesystem cannot lock (some NFS, SMB and FUSE mounts), it warns on
// stderr and returns without a lock.
func LockDir(dir string) (func(), error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("locking %s: %w", dir, err)
	}

	fd := int(f.Fd()) //nolint:gosec // file descriptors fit in int
	deadline := time.Now().Add(lockWait)

	for {
		err = flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			// Closing the descriptor releases the lock.
			return func() { _ = f.Close() }, nil
		}

		if cannotLock(err) {
			_ = f.Close()

			return skipLock(dir, err), nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			_ = f.Close()

			return nil, fmt.Errorf("locking %s: %w", dir, err)
		}

		if time.Now().After(deadline) {
			_ = f.Close()

			return nil, fmt.Errorf("timed out waiting for another yat command to finish in %s", dir)
		}

		time.Sleep(lockPoll)
	}
}

// cannotLock reports whether err, from flock, means the filesystem does not
// support locking rather than that another process holds the lock.
func cannotLock(err error) bool {
	for _, errno := range []syscall.Errno{syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.ENOLCK, syscall.EROFS} {
		if errors.Is(err, errno) {
			return true
		}
	}

	return false
}
