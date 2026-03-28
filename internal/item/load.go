package item

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// LoadAll discovers and parses all .md files from the given directory (recursively).
// Files without a valid id field in frontmatter are silently skipped.
func LoadAll(dir string) ([]*Item, error) {
	return LoadAllWithOptions(dir, ParseOptions{})
}

// LoadAllWithOptions is like LoadAll but accepts custom parsing options.
func LoadAllWithOptions(dir string, opts ParseOptions) ([]*Item, error) {
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
