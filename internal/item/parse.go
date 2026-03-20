package item

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseOptions controls how frontmatter is interpreted.
type ParseOptions struct {
	// Aliases maps alias key → canonical key (inverted from config).
	Aliases map[string]string
	// ValidStatuses, when non-nil, replaces the built-in Status.Valid() check.
	ValidStatuses StatusSet
	// DefaultStatus is used when the status field is empty. Zero value means "draft".
	DefaultStatus Status
}

var frontmatterDelimiter = []byte("---")

// ErrMissingID is returned when a file has item-like frontmatter but no id field.
var ErrMissingID = errors.New("item-like frontmatter has no id field")

// ParseFile reads a markdown file and extracts the YAML frontmatter into an Item.
// The body (everything after the closing ---) is stored in Body.
// Returns nil if the file has no valid frontmatter or no id field.
func ParseFile(path string) (*Item, error) {
	return ParseFileWithOptions(path, ParseOptions{})
}

// ParseFileWithOptions is like ParseFile but accepts custom parsing options.
func ParseFileWithOptions(path string, opts ParseOptions) (*Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return ParseWithOptions(data, path, opts)
}

// Parse extracts an Item from raw file bytes.
// Returns nil (no error) if the file has no valid frontmatter.
// Returns ErrMissingID (wrapped) if frontmatter contains item fields but no id.
// Returns an error if status or priority values are not recognized.
func Parse(data []byte, path string) (*Item, error) {
	return ParseWithOptions(data, path, ParseOptions{})
}

// ParseWithOptions is like Parse but accepts custom parsing options for
// field aliasing and custom status validation.
func ParseWithOptions(data []byte, path string, opts ParseOptions) (*Item, error) {
	fm, body, ok := splitFrontmatter(data)
	if !ok {
		return nil, nil
	}

	if hasDirective(fm, "ignore") {
		return nil, nil
	}

	fm, err := remapAliases(fm, opts.Aliases, path)
	if err != nil {
		return nil, err
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

	if err := validateStatus(&item, opts, path); err != nil {
		return nil, err
	}

	if !item.Priority.Valid() {
		return nil, fmt.Errorf("invalid priority %q in %s: must be Critical, High, Medium, or Low", item.Priority, path)
	}

	item.FilePath = path
	item.Body = string(body)

	return &item, nil
}

// remapAliases rewrites aliased frontmatter keys to their canonical names.
func remapAliases(fm []byte, aliases map[string]string, path string) ([]byte, error) {
	if len(aliases) == 0 {
		return fm, nil
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(fm, &raw); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	for alias, canonical := range aliases {
		if v, ok := raw[alias]; ok {
			if _, exists := raw[canonical]; !exists {
				raw[canonical] = v
				delete(raw, alias)
			}
		}
	}

	remapped, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("re-marshaling frontmatter in %s: %w", path, err)
	}

	return remapped, nil
}

// validateStatus checks and normalizes the status field on an item.
func validateStatus(item *Item, opts ParseOptions, path string) error {
	if opts.ValidStatuses != nil {
		if !opts.ValidStatuses.Contains(item.Status) {
			valid := make([]string, 0, len(opts.ValidStatuses))
			for s := range opts.ValidStatuses {
				valid = append(valid, string(s))
			}

			return fmt.Errorf("invalid status %q in %s: must be one of %v", item.Status, path, valid)
		}
	} else if !item.Status.Valid() {
		return fmt.Errorf("invalid status %q in %s: must be draft, in-progress, or done", item.Status, path)
	}

	if item.Status == "" {
		if opts.DefaultStatus != "" {
			item.Status = opts.DefaultStatus
		} else {
			item.Status = StatusDraft
		}
	}

	return nil
}

func looksLikeItem(item *Item) bool {
	return item.Status != "" || item.Type != "" || item.Priority != "" || len(item.Dependencies) > 0
}

// hasDirective checks whether the frontmatter contains a specific yat directive.
// The yat field is a comma-delimited list of directives (e.g. "yat: ignore" or "yat: ignore, draft").
func hasDirective(fm []byte, directive string) bool {
	var meta struct {
		Yat string `yaml:"yat"`
	}

	if err := yaml.Unmarshal(fm, &meta); err != nil || meta.Yat == "" {
		return false
	}

	for _, d := range strings.Split(meta.Yat, ",") {
		if strings.TrimSpace(d) == directive {
			return true
		}
	}

	return false
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

	// The closing --- must be on its own line (followed by newline, CR, or EOF).
	afterClose := rest[idx+len("\n---"):]
	if len(afterClose) > 0 && afterClose[0] != '\n' && afterClose[0] != '\r' {
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
