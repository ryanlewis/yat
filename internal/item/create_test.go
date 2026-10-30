package item

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCreateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	if err := CreateFile(path, []byte("one"), 0o600); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	if err := CreateFile(path, []byte("two"), 0o600); !errors.Is(err, ErrFileExists) {
		t.Errorf("second CreateFile = %v, want ErrFileExists", err)
	}

	if data, _ := os.ReadFile(path); string(data) != "one" {
		t.Errorf("file = %q, want it unchanged", data)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("want only new.md left, got %d entries", len(entries))
	}
}

func TestCreateFile_MissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "new.md")

	err := CreateFile(path, []byte("one"), 0o600)
	if err == nil {
		t.Fatal("CreateFile into a missing dir = nil, want error")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("want dir left empty, got %d entries", len(entries))
	}
}

func TestCreateFile_UnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := CreateFile(filepath.Join(dir, "new.md"), []byte("one"), 0o600)
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("err = %v, want fs.ErrPermission", err)
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("want dir left empty, got %d entries", len(entries))
	}
}

func TestCreateExclusive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	if err := createExclusive(path, []byte("one"), 0o666); err != nil {
		t.Fatalf("createExclusive: %v", err)
	}

	if data, _ := os.ReadFile(path); string(data) != "one" {
		t.Errorf("file = %q, want %q", data, "one")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// writeNew chmods after open, so the umask does not narrow the mode.
	if info.Mode().Perm() != 0o666 {
		t.Errorf("mode = %v, want 0666", info.Mode().Perm())
	}
}

func TestCreateExclusive_ExistingFileKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := createExclusive(path, []byte("two"), 0o600)
	if !errors.Is(err, ErrFileExists) {
		t.Errorf("createExclusive over a file = %v, want ErrFileExists", err)
	}

	if err != nil && !strings.Contains(err.Error(), path) {
		t.Errorf("err = %q, want it to name %s", err, path)
	}

	if data, _ := os.ReadFile(path); string(data) != "original" {
		t.Errorf("file = %q, want it unchanged", data)
	}
}

func TestCreateExclusive_MissingParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "new.md")

	err := createExclusive(path, []byte("one"), 0o600)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestWriteNew_WriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.md")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path) // read-only, so Write fails
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	if writeNew(f, []byte("one"), 0o644) == nil {
		t.Fatal("writeNew on a read-only file = nil, want error")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// A failed write stops before the chmod.
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 left alone", info.Mode().Perm())
	}
}

func TestWriteNew_ClosedFile(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "closed.md"))
	if err != nil {
		t.Fatal(err)
	}

	_ = f.Close()

	if err := writeNew(f, nil, 0o600); !errors.Is(err, os.ErrClosed) {
		t.Errorf("writeNew on a closed file = %v, want os.ErrClosed", err)
	}
}

// A file whose name is near the 255-byte name limit can still be created: the
// temp file written next to it must not need a longer name.
func TestCreateFile_LongFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), strings.Repeat("a", 247)+".md")

	if err := CreateFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("CreateFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != "x" {
		t.Errorf("got %q, %v; want \"x\"", data, err)
	}
}

// failLink makes linkFile fail with errno for the rest of the test.
func failLink(t *testing.T, errno syscall.Errno) {
	t.Helper()

	orig := linkFile
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: errno}
	}

	t.Cleanup(func() { linkFile = orig })
}

func TestCreateFile_LinkErrorReturned(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.ENOSPC, syscall.EACCES, syscall.EROFS, syscall.EIO} {
		t.Run(errno.Error(), func(t *testing.T) {
			failLink(t, errno)

			dir := t.TempDir()
			path := filepath.Join(dir, "new.md")

			err := CreateFile(path, []byte("one"), 0o600)
			if !errors.Is(err, errno) {
				t.Fatalf("err = %v, want %v", err, errno)
			}

			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("want dir left empty, got %d entries", len(entries))
			}
		})
	}
}

func TestCreateFile_LinkUnsupportedFallsBack(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EPERM, syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.ENOSYS} {
		t.Run(errno.Error(), func(t *testing.T) {
			failLink(t, errno)

			dir := t.TempDir()
			path := filepath.Join(dir, "new.md")

			if err := CreateFile(path, []byte("one"), 0o600); err != nil {
				t.Fatalf("CreateFile: %v", err)
			}

			if data, _ := os.ReadFile(path); string(data) != "one" {
				t.Errorf("file = %q, want %q", data, "one")
			}

			if entries, _ := os.ReadDir(dir); len(entries) != 1 {
				t.Errorf("want only new.md left, got %d entries", len(entries))
			}

			if err := CreateFile(path, []byte("two"), 0o600); !errors.Is(err, ErrFileExists) {
				t.Errorf("second CreateFile = %v, want ErrFileExists", err)
			}
		})
	}
}
