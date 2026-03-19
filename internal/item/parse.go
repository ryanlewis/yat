package item

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var frontmatterDelimiter = []byte("---")

// ErrMissingID is returned when a file has item-like frontmatter but no id field.
var ErrMissingID = errors.New("item-like frontmatter has no id field")

// ParseFile reads a markdown file and extracts the YAML frontmatter into an Item.
// The body (everything after the closing ---) is stored in Body.
// Returns nil if the file has no valid frontmatter or no id field.
func ParseFile(path string) (*Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return Parse(data, path)
}

// Parse extracts an Item from raw file bytes.
// Returns nil (no error) if the file has no valid frontmatter.
// Returns ErrMissingID (wrapped) if frontmatter contains item fields but no id.
// Returns an error if status or priority values are not recognized.
func Parse(data []byte, path string) (*Item, error) {
	fm, body, ok := splitFrontmatter(data)
	if !ok {
		return nil, nil
	}

	var item Item
	if err := yaml.Unmarshal(fm, &item); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	if item.ID == "" {
		if looksLikeItem(&item) {
			return nil, fmt.Errorf("%w: %s", ErrMissingID, path)
		}

		return nil, nil
	}

	if !item.Status.Valid() {
		return nil, fmt.Errorf("invalid status %q in %s: must be draft, in-progress, or done", item.Status, path)
	}

	if item.Status == "" {
		item.Status = StatusDraft
	}

	if !item.Priority.Valid() {
		return nil, fmt.Errorf("invalid priority %q in %s: must be Critical, High, Medium, or Low", item.Priority, path)
	}

	item.FilePath = path
	item.Body = string(body)

	return &item, nil
}

func looksLikeItem(item *Item) bool {
	return item.Status != "" || item.Type != "" || item.Priority != "" || len(item.Dependencies) > 0
}

// splitFrontmatter splits file content at the opening and closing --- delimiters.
// Returns the frontmatter bytes, body bytes, and whether a valid split was found.
func splitFrontmatter(data []byte) (fm, body []byte, ok bool) {
	data = bytes.TrimLeft(data, "\n\r")

	if !bytes.HasPrefix(data, frontmatterDelimiter) {
		return nil, nil, false
	}

	// The opening --- must be followed by a newline (LF or CRLF), not arbitrary text.
	rest := data[len(frontmatterDelimiter):]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return nil, nil, false
	}
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return nil, nil, false
	}

	fm = rest[:idx]

	// Body starts after the closing --- and its trailing newline
	body = rest[idx+len("\n---"):]
	if len(body) > 0 && body[0] == '\r' {
		body = body[1:]
	}
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}

	return fm, body, true
}
