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

// warnOut receives one line per .md file that LoadAll skips with a warning.
var warnOut io.Writer = os.Stderr

// LoadAll discovers and parses all .md files from the given directory (recursively).
// Files without a valid id field in frontmatter are silently skipped. Files that
// cannot be read, or whose frontmatter is not a YAML mapping, are skipped with a
// warning on stderr.
func LoadAll(dir string) ([]*Item, error) {
	return LoadAllWithOptions(dir, ParseOptions{})
}

// LoadAllWithOptions is like LoadAll but accepts custom parsing options.
func LoadAllWithOptions(dir string, opts ParseOptions) ([]*Item, error) {
	return loadAll(dir, opts, warnSkip)
}

// ReloadWithOptions is like LoadAllWithOptions but warns about no file, for a
// second load of a directory whose warnings were already printed.
func ReloadWithOptions(dir string, opts ParseOptions) ([]*Item, error) {
	return loadAll(dir, opts, func(string, error) {})
}

func loadAll(dir string, opts ParseOptions, warnSkip func(path string, err error)) ([]*Item, error) {
	var items []*Item

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		item, parseErr := ParseFileWithOptions(path, opts)
		if parseErr != nil {
			if errors.Is(parseErr, ErrMissingID) {
				return nil
			}

			// A *fs.PathError means the file could not be read (permissions,
			// a broken symlink, a symlink to a directory).
			var pathErr *fs.PathError
			if errors.As(parseErr, &pathErr) {
				warnSkip(path, pathErr.Err)

				return nil
			}

			if errors.Is(parseErr, ErrNotMapping) {
				warnSkip(path, ErrNotMapping)

				return nil
			}

			return fmt.Errorf("loading %s: %w", path, parseErr)
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

// warnSkip writes one warning line saying path was skipped and why.
func warnSkip(path string, err error) {
	_, _ = fmt.Fprintf(warnOut, "warning: skipping %s: %v\n", path, err)
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
