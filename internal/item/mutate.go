package item

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"regexp"
)

// MutateOptions controls how SetStatusWithOptions behaves.
type MutateOptions struct {
	// FieldName is the frontmatter key to rewrite (defaults to "status").
	FieldName string
	// ValidStatuses, when non-nil, replaces the built-in Status.Valid() check.
	ValidStatuses StatusSet
}

const filePermissions fs.FileMode = 0o600

// SetStatus updates the status field in the frontmatter of the given file.
// It performs a targeted replacement of the status: line within the frontmatter only,
// preserving the body byte-for-byte. Returns an error if the file contains no
// status field in its frontmatter.
func SetStatus(path string, newStatus Status) error {
	return SetStatusWithOptions(path, newStatus, MutateOptions{})
}

// SetStatusWithOptions is like SetStatus but supports a custom field name and
// custom status validation via MutateOptions.
func SetStatusWithOptions(path string, newStatus Status, opts MutateOptions) error {
	fieldName := opts.FieldName
	if fieldName == "" {
		fieldName = "status"
	}

	if opts.ValidStatuses != nil {
		if newStatus == "" || !opts.ValidStatuses.Contains(newStatus) {
			return fmt.Errorf("invalid status %q", newStatus)
		}
	} else if newStatus == "" || !newStatus.Valid() {
		return fmt.Errorf("invalid status %q: must be draft, in-progress, or done", newStatus)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	fm, body, ok := splitFrontmatter(data)
	if !ok {
		return fmt.Errorf("no frontmatter found in %s", path)
	}

	fieldRe := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(fieldName) + `:\s*\S+`)

	if !fieldRe.Match(fm) {
		return fmt.Errorf("no %s field found in frontmatter of %s", fieldName, path)
	}

	replacement := fieldName + ": " + string(newStatus)
	updatedFM := fieldRe.ReplaceAllLiteral(fm, []byte(replacement))

	var buf bytes.Buffer
	buf.WriteString("---")
	buf.Write(updatedFM)
	buf.WriteString("\n---\n")
	buf.Write(body)

	if err := os.WriteFile(path, buf.Bytes(), filePermissions); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
