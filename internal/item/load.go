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
// is not a YAML mapping or never closes are skipped with a warning on stderr. The
// paths of the files and subdirectories that could not be read are returned as
// skipped, as an item they hold is missing from the result.
func LoadAll(dir string) (items []*Item, skipped []string, err error) {
	return LoadAllWithOptions(dir, ParseOptions{})
}

// LoadAllWithOptions is like LoadAll but accepts custom parsing options.
func LoadAllWithOptions(dir string, opts ParseOptions) ([]*Item, []string, error) {
	var (
		items   []*Item
		skipped []string
	)

	skip := func(path string, err error) {
		// Frontmatter that is not a mapping has no id, so the file holds no item.
		// A file that never closes its frontmatter is most likely a note that
		// opens with a --- rule, so it does not block picking an id either.
		if !errors.Is(err, ErrNotMapping) && !errors.Is(err, ErrUnclosedFrontmatter) {
			skipped = append(skipped, path)
		}

		warnSkip(path, err)
	}

	root := walkRoot(dir)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A subdirectory that cannot be read is skipped like an unreadable
			// file; the root itself still fails the load.
			if path != root && d != nil && d.IsDir() {
				skip(path, unwrapPathErr(err))

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

		item, err := loadFile(path, opts, skip)
		if err != nil {
			return err
		}

		if item != nil {
			items = append(items, item)
		}

		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walking directory %s: %w", dir, err)
	}

	return items, skipped, nil
}

// walkRoot returns dir with a trailing separator, so that WalkDir follows the
// root when it is a symlink to a directory. Symlinks inside the tree are still
// not followed. Item paths are unchanged, as WalkDir joins and cleans them.
// A bare volume name such as C: is left alone, as C:\ names a different directory.
func walkRoot(dir string) string {
	if dir == "" || dir == filepath.VolumeName(dir) || os.IsPathSeparator(dir[len(dir)-1]) {
		return dir
	}

	return dir + string(filepath.Separator)
}

// loadFile parses one .md file for LoadAllWithOptions. It returns a nil item
// and nil error for files that are skipped, with a warning where LoadAll says so.
func loadFile(path string, opts ParseOptions, warnSkip func(path string, err error)) (*Item, error) {
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

	for _, skipErr := range []error{ErrNotMapping, ErrUnclosedFrontmatter} {
		if errors.Is(err, skipErr) {
			warnSkip(path, skipErr)

			return nil, nil
		}
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
