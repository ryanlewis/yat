//go:build unix

package item

import (
	"io/fs"
	"os"
	"slices"
	"syscall"
	"testing"
)

// These tests swap chownFile, so none of them run in parallel.

// sysInfo is a FileInfo whose Sys returns sys.
type sysInfo struct {
	fs.FileInfo
	sys any
}

func (i sysInfo) Sys() any { return i.sys }

// recordChown makes chownFile record its uid and gid arguments, failing with
// err when the uid is not -1, for the rest of the test.
func recordChown(t *testing.T, err error) *[][2]int {
	t.Helper()

	var calls [][2]int

	orig := chownFile
	chownFile = func(_ *os.File, uid, gid int) error {
		calls = append(calls, [2]int{uid, gid})
		if uid != -1 {
			return err
		}

		return nil
	}

	t.Cleanup(func() { chownFile = orig })

	return &calls
}

// ownerFixture returns an open temp file and an orig FileInfo whose owner is
// the file's own, moved by du and dg.
func ownerFixture(t *testing.T, du, dg uint32) (*os.File, fs.FileInfo) {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "*.tmp")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = f.Close() })

	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}

	st := *info.Sys().(*syscall.Stat_t)
	st.Uid += du
	st.Gid += dg

	return f, sysInfo{FileInfo: info, sys: &st}
}

func TestKeepOwner(t *testing.T) {
	tests := []struct {
		name    string
		du, dg  uint32
		chown   error
		closeIt bool
		noStat  bool
		want    func(st syscall.Stat_t) [][2]int
	}{
		{
			name: "same owner",
			want: func(syscall.Stat_t) [][2]int { return nil },
		},
		{
			name: "other owner",
			du:   1,
			dg:   1,
			want: func(st syscall.Stat_t) [][2]int { return [][2]int{{int(st.Uid), int(st.Gid)}} },
		},
		{
			name:  "chown refused keeps the group",
			du:    1,
			dg:    1,
			chown: syscall.EPERM,
			want: func(st syscall.Stat_t) [][2]int {
				return [][2]int{{int(st.Uid), int(st.Gid)}, {-1, int(st.Gid)}}
			},
		},
		{
			name:    "stat error",
			du:      1,
			closeIt: true,
			want:    func(syscall.Stat_t) [][2]int { return nil },
		},
		{
			name:   "no unix owner",
			du:     1,
			noStat: true,
			want:   func(syscall.Stat_t) [][2]int { return nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := recordChown(t, tt.chown)

			f, orig := ownerFixture(t, tt.du, tt.dg)
			want := tt.want(*orig.Sys().(*syscall.Stat_t))

			if tt.closeIt {
				_ = f.Close()
			}

			if tt.noStat {
				orig = sysInfo{FileInfo: orig, sys: nil}
			}

			keepOwner(f, orig)

			if !slices.Equal(*calls, want) {
				t.Errorf("chown calls = %v, want %v", *calls, want)
			}
		})
	}
}
