//go:build !unix

package item

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// lockName is the file LockDir creates in the directory it locks. It is hidden
// and not .md, so LoadAll never picks it up.
const lockName = ".yat.lock"

const lockPerm fs.FileMode = 0o600

// lockStale is the age at which a lock is taken to be left by a crashed run.
var lockStale = 5 * time.Second

// LockDir takes a short-lived lock on dir by creating lockName in it with
// O_EXCL, waiting while another process holds it. A lock older than lockStale
// is taken to be left behind by a run that crashed and is removed. Call the
// returned function to release the lock.
func LockDir(dir string) (func(), error) {
	path := filepath.Join(dir, lockName)
	deadline := time.Now().Add(lockWait)

	for {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, lockPerm)
		if err == nil {
			_ = f.Close()

			return func() { _ = os.Remove(path) }, nil
		}

		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("locking %s: %w", dir, err)
		}

		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > lockStale {
			_ = os.Remove(path)

			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for %s; if no other yat command is running, remove it", path)
		}

		time.Sleep(lockPoll)
	}
}
