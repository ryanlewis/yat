//go:build unix

package item

import (
	"io/fs"
	"os"
	"syscall"
)

// keepOwner gives f the owner and group of orig where they differ, ignoring
// failure: only root can give a file away, and a group change can be refused.
func keepOwner(f *os.File, orig fs.FileInfo) {
	want, ok := orig.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}

	info, err := f.Stat()
	if err != nil {
		return
	}

	if got, ok := info.Sys().(*syscall.Stat_t); ok && (got.Uid != want.Uid || got.Gid != want.Gid) {
		_ = f.Chown(int(want.Uid), int(want.Gid)) //nolint:errcheck // best effort, see above
	}
}
