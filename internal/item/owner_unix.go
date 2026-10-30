//go:build unix

package item

import (
	"io/fs"
	"os"
	"syscall"
)

// chownFile is (*os.File).Chown, swapped in tests to make it fail on demand.
var chownFile = (*os.File).Chown

// keepOwner gives f the owner and group of orig where they differ, ignoring
// failure. Only root can give a file away, so when that fails it keeps at least
// the group, which works if the user is in it.
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
		if chownFile(f, int(want.Uid), int(want.Gid)) != nil {
			_ = chownFile(f, -1, int(want.Gid)) //nolint:errcheck // best effort, see above
		}
	}
}
