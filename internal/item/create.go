package item

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// ErrFileExists is returned by CreateFile when a file is already at the path.
var ErrFileExists = errors.New("file already exists")

// linkFile is os.Link, swapped in tests to make the link fail on demand.
var linkFile = os.Link

// CreateFile writes data to a new file at path with mode perm, refusing to
// replace a file that is already there. Like writeFileAtomic it writes a hidden
// temp file next to the file first (LoadAll never picks it up), so a crash or a
// full disk never leaves a partial file at path.
func CreateFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".yat-*.tmp")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	defer os.Remove(tmpName) // path keeps the data once linked

	if err = writeNew(tmp, data, perm); err != nil {
		_ = tmp.Close()

		return err
	}

	// Link, unlike Rename, fails when path exists, so a file is never replaced.
	err = linkFile(tmpName, path)

	switch {
	case err == nil:
		syncDir(dir)

		return nil
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf("%w: %s", ErrFileExists, path)
	case linkUnsupported(err):
		// The filesystem does not support hard links; write the file directly.
		return createExclusive(path, data, perm)
	default:
		return err // an *os.LinkError, naming both paths
	}
}

// linkUnsupported reports whether a link error means the filesystem has no hard
// links (FAT, exFAT, some FUSE and SMB mounts), rather than a real failure such
// as a full disk, which a direct write would only hit again, without atomicity.
func linkUnsupported(err error) bool {
	return errors.Is(err, errors.ErrUnsupported) || errors.Is(err, syscall.EPERM)
}

// writeNew writes data to f, gives it mode perm, syncs it to disk and closes it.
func writeNew(f *os.File, data []byte, perm fs.FileMode) error {
	if _, err := f.Write(data); err != nil {
		return err
	}

	if err := f.Chmod(perm); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return f.Close()
}

// createExclusive writes data to a new file at path, failing if one exists, and
// syncs the parent directory so the new entry survives a crash.
func createExclusive(path string, data []byte, perm fs.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s", ErrFileExists, path)
		}

		return err
	}

	if err := writeNew(f, data, perm); err != nil {
		_ = f.Close()
		_ = os.Remove(path)

		return err
	}

	syncDir(filepath.Dir(path))

	return nil
}
