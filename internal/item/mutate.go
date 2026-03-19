package item

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"regexp"
)

const filePermissions fs.FileMode = 0o600

var statusLineRe = regexp.MustCompile(`(?m)^status:\s*\S+`)

// SetStatus updates the status field in the frontmatter of the given file.
// It performs a targeted replacement of the status: line within the frontmatter only,
// preserving the body byte-for-byte. Returns an error if the file contains no
// status field in its frontmatter.
func SetStatus(path string, newStatus Status) error {
	if newStatus == "" || !newStatus.Valid() {
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

	if !statusLineRe.Match(fm) {
		return fmt.Errorf("no status field found in frontmatter of %s", path)
	}

	replacement := "status: " + string(newStatus)
	updatedFM := statusLineRe.ReplaceAllLiteral(fm, []byte(replacement))

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
