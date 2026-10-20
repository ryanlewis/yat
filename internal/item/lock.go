package item

import (
	"fmt"
	"time"
)

var (
	lockWait = 10 * time.Second      // how long LockDir waits for another holder
	lockPoll = 10 * time.Millisecond // how often LockDir retries
)

// skipLock writes one warning that dir could not be locked and returns a
// release that does nothing, for a folder or filesystem where locking cannot
// work: the command goes on as it did before yat took a lock.
func skipLock(dir string, err error) func() {
	_, _ = fmt.Fprintf(warnOut, "warning: could not lock %s: %v; continuing without a lock\n", dir, err)

	return func() {}
}
