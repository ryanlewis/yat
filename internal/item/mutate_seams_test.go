package item

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
)

// These tests swap package seams, so none of them run in parallel.

// The item contents before and after a write.
const (
	seamOld = "old"
	seamNew = "new"
)

// writeSeamFixture writes the old contents to a new item file and returns its
// path and file info.
func writeSeamFixture(t *testing.T) (path string, info os.FileInfo) {
	t.Helper()

	path = writeMutateFixture(t, "item.md", seamOld)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	return path, info
}

// assertOnlyItem fails unless the item file is the only file in its directory,
// so no temp file was left behind.
func assertOnlyItem(t *testing.T, path string) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}

	if !slices.Equal(names, []string{filepath.Base(path)}) {
		t.Errorf("dir holds %v, want only %s", names, filepath.Base(path))
	}
}

// assertSameInode fails unless path is still the file described by before, so
// it was written in place rather than replaced.
func assertSameInode(t *testing.T, path string, before os.FileInfo) {
	t.Helper()

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if !os.SameFile(before, after) {
		t.Error("file was replaced, want it written in place")
	}
}

// failCreateTemp makes createTemp fail with errno for the rest of the test.
func failCreateTemp(t *testing.T, errno syscall.Errno) {
	t.Helper()

	orig := createTemp
	createTemp = func(dir, pattern string) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: filepath.Join(dir, pattern), Err: errno}
	}

	t.Cleanup(func() { createTemp = orig })
}

// failRename makes renameFile fail with errno for the rest of the test.
func failRename(t *testing.T, errno syscall.Errno) {
	t.Helper()

	orig := renameFile
	renameFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: errno}
	}

	t.Cleanup(func() { renameFile = orig })
}

func TestWriteFileAtomic_CreateTempErrorReturned(t *testing.T) {
	failCreateTemp(t, syscall.ENOSPC)

	path, _ := writeSeamFixture(t)

	if err := writeFileAtomic(path, []byte(seamNew)); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("err = %v, want ENOSPC", err)
	}

	if got := readMutateFile(t, path); got != seamOld {
		t.Errorf("contents = %q, want the old contents", got)
	}
}

func TestWriteFileAtomic_CreateTempDeniedWritesInPlace(t *testing.T) {
	failCreateTemp(t, syscall.EACCES)

	path, before := writeSeamFixture(t)

	if err := writeFileAtomic(path, []byte(seamNew)); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	if got := readMutateFile(t, path); got != seamNew {
		t.Errorf("contents = %q, want %q", got, seamNew)
	}

	assertSameInode(t, path, before)
}

// When writing the temp file fails, the temp file is removed and the item is
// left alone.
func TestWriteFileAtomic_WriteErrorRemovesTemp(t *testing.T) {
	orig := createTemp
	createTemp = func(dir, pattern string) (*os.File, error) {
		f, err := orig(dir, pattern)
		if err == nil {
			err = f.Close() // a closed file fails the write
		}

		return f, err
	}

	t.Cleanup(func() { createTemp = orig })

	path, _ := writeSeamFixture(t)

	if err := writeFileAtomic(path, []byte(seamNew)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("err = %v, want ErrClosed", err)
	}

	if got := readMutateFile(t, path); got != seamOld {
		t.Errorf("contents = %q, want the old contents", got)
	}

	assertOnlyItem(t, path)
}

func TestWriteFileAtomic_RenameErrorReturned(t *testing.T) {
	failRename(t, syscall.EIO)

	path, _ := writeSeamFixture(t)

	if err := writeFileAtomic(path, []byte(seamNew)); !errors.Is(err, syscall.EIO) {
		t.Fatalf("err = %v, want EIO", err)
	}

	if got := readMutateFile(t, path); got != seamOld {
		t.Errorf("contents = %q, want the old contents", got)
	}

	assertOnlyItem(t, path)
}

// A rename that cannot work here (a bind-mounted file gives EXDEV or EBUSY)
// falls back to writing the item in place, and the temp file is removed.
func TestWriteFileAtomic_RenameRefusedWritesInPlace(t *testing.T) {
	failRename(t, syscall.EXDEV)

	path, before := writeSeamFixture(t)

	if err := writeFileAtomic(path, []byte(seamNew)); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	if got := readMutateFile(t, path); got != seamNew {
		t.Errorf("contents = %q, want %q", got, seamNew)
	}

	assertSameInode(t, path, before)
	assertOnlyItem(t, path)
}
