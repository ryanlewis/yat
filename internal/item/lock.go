package item

import "time"

var (
	lockWait = 10 * time.Second      // how long LockDir waits for another holder
	lockPoll = 10 * time.Millisecond // how often LockDir retries
)
