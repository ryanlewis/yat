package item

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// warnOut receives one line per .md file or subdirectory that LoadAll skips with a warning.
var warnOut io.Writer = os.Stderr

// LoadAll discovers and parses all .md files from the given directory (recursively).
// Files without a valid id field in frontmatter are silently skipped. Files that
// cannot be read, subdirectories that cannot be read, and files whose frontmatter
// is not a YAML mapping are skipped with a warning on stderr.
func LoadAll(dir string) ([]*Item, error) {
	return LoadAllWithOptions(dir, ParseOptions{})
}

// LoadAllWithOptions is like LoadAll but accepts custom parsing options.
func LoadAllWithOptions(dir string, opts ParseOptions) ([]*Item, error) {
	var items []*Item

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A subdirectory that cannot be read is skipped like an unreadable
			// file; the root itself still fails the load.
			if path != dir && d != nil && d.IsDir() {
				warnSkip(path, unwrapPathErr(err))

				return fs.SkipDir
			}

			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		item, err := loadFile(path, opts)
		if err != nil {
			return err
		}

		if item != nil {
			items = append(items, item)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory %s: %w", dir, err)
	}

	return items, nil
}

// loadFile parses one .md file for LoadAllWithOptions. It returns a nil item
// and nil error for files that are skipped, with a warning where LoadAll says so.
func loadFile(path string, opts ParseOptions) (*Item, error) {
	item, err := ParseFileWithOptions(path, opts)
	if err == nil {
		return item, nil
	}

	if errors.Is(err, ErrMissingID) {
		return nil, nil
	}

	// A *fs.PathError means the file could not be read (permissions,
	// a broken symlink, a symlink to a directory).
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		warnSkip(path, pathErr.Err)

		return nil, nil
	}

	if errors.Is(err, ErrNotMapping) {
		warnSkip(path, ErrNotMapping)

		return nil, nil
	}

	return nil, fmt.Errorf("loading %s: %w", path, err)
}

// warnSkip writes one warning line saying path was skipped and why.
func warnSkip(path string, err error) {
	_, _ = fmt.Fprintf(warnOut, "warning: skipping %s: %v\n", path, err)
}

// unwrapPathErr returns the underlying error of a *fs.PathError, or err itself,
// so the warning does not repeat the path.
func unwrapPathErr(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}

	return err
}

// Index builds a map from item ID to Item for fast lookup.
// Returns an error if duplicate IDs are found across files.
func Index(items []*Item) (map[string]*Item, error) {
	m := make(map[string]*Item, len(items))

	for _, item := range items {
		if existing, ok := m[item.ID]; ok {
			return nil, fmt.Errorf("duplicate id %q found in %s and %s", item.ID, existing.FilePath, item.FilePath)
		}

		m[item.ID] = item
	}

	return m, nil
}
