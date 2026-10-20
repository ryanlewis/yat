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
	// DefaultStatus is used when the status field is empty. Zero value means "todo".
	DefaultStatus Status
}

var (
	frontmatterDelimiter = []byte("---")
	utf8BOM              = []byte("\xef\xbb\xbf")
)

// ErrMissingID is returned when a file has item-like frontmatter but no id field.
var ErrMissingID = errors.New("item-like frontmatter has no id field")

// ErrNotMapping is returned when a file's frontmatter is valid YAML but not a
// mapping (e.g. a line of prose between --- delimiters), so it cannot be an item.
var ErrNotMapping = errors.New("frontmatter is not a YAML mapping")

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
// Returns ErrNotMapping (wrapped) if the frontmatter is YAML but not a mapping.
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
		if isNonMapping(fm) {
			return nil, fmt.Errorf("%w: %s", ErrNotMapping, path)
		}

		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	delete(item.Extra, "yat")
	if len(item.Extra) == 0 {
		item.Extra = nil
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

	if item.Title == "" {
		item.Title = extractHeading(body)
	}

	return &item, nil
}

// extractHeading returns the text of the first ATX heading (h1–h3) in the body, or "".
// Headings inside fenced code blocks (``` or ~~~) are ignored.
func extractHeading(body []byte) string {
	var fence []byte // marker of the open fence, nil outside one

	for _, line := range bytes.Split(body, []byte("\n")) {
		trimmed := bytes.TrimSpace(line)

		if fence != nil {
			// Only a bare run of the same character, at least as long, closes the fence.
			if run := fenceMarker(trimmed); run != nil && len(run) == len(trimmed) &&
				run[0] == fence[0] && len(run) >= len(fence) {
				fence = nil
			}

			continue
		}

		if fence = fenceMarker(trimmed); fence != nil {
			continue
		}

		for _, prefix := range []string{"# ", "## ", "### "} {
			if bytes.HasPrefix(trimmed, []byte(prefix)) {
				return string(bytes.TrimSpace(trimmed[len(prefix):]))
			}
		}
	}

	return ""
}

// minFenceLen is the shortest run of ` or ~ that opens a code fence.
const minFenceLen = 3

// fenceMarker returns the leading run of ` or ~ in line when it is long enough
// to open a code fence, or nil.
func fenceMarker(line []byte) []byte {
	if len(line) == 0 || (line[0] != '`' && line[0] != '~') {
		return nil
	}

	n := 0
	for n < len(line) && line[n] == line[0] {
		n++
	}

	if n < minFenceLen {
		return nil
	}

	return line[:n]
}

// remapAliases rewrites aliased frontmatter keys to their canonical names.
func remapAliases(fm []byte, aliases map[string]string, path string) ([]byte, error) {
	if len(aliases) == 0 {
		return fm, nil
	}

	// Rename keys in the node tree rather than round-tripping through a map, so
	// values keep their source text (007, 1.10) and key order and anchors survive.
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		return nil, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fm, nil
	}

	mapping := doc.Content[0]
	keys := make(map[string]*yaml.Node, len(mapping.Content)/2)

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keys[mapping.Content[i].Value] = mapping.Content[i]
	}

	for alias, canonical := range aliases {
		if key, ok := keys[alias]; ok {
			if _, exists := keys[canonical]; !exists {
				key.Value = canonical
				keys[canonical] = key
				delete(keys, alias)
			}
		}
	}

	remapped, err := yaml.Marshal(&doc)
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
		return fmt.Errorf("invalid status %q in %s: must be todo, in-progress, or done", item.Status, path)
	}

	if item.Status == "" {
		if opts.DefaultStatus != "" {
			item.Status = opts.DefaultStatus
		} else {
			item.Status = StatusTodo
		}
	}

	return nil
}

// isNonMapping reports whether fm is valid YAML whose top-level node is not a
// mapping. Invalid YAML reports false, so syntax errors stay errors.
func isNonMapping(fm []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil || len(doc.Content) == 0 {
		return false
	}

	return doc.Content[0].Kind != yaml.MappingNode
}

func looksLikeItem(item *Item) bool {
	return item.Status != "" || item.Type != "" || item.Priority != "" || len(item.Dependencies) > 0
}

// hasDirective checks whether the frontmatter contains a specific yat directive.
// The yat field is a comma-delimited list of directives (e.g. "yat: ignore" or "yat: ignore, notrack").
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
	fmStart, fmEnd, bodyStart, ok := frontmatterBounds(data)
	if !ok {
		return nil, nil, false
	}

	return data[fmStart:fmEnd], data[bodyStart:], true
}

// frontmatterBounds locates the frontmatter in data. data[fmStart:fmEnd] is the
// frontmatter (starting with the newline after the opening ---) and
// data[bodyStart:] is the body. A leading UTF-8 BOM and blank lines are skipped.
func frontmatterBounds(data []byte) (fmStart, fmEnd, bodyStart int, ok bool) {
	start := len(data) - len(bytes.TrimLeft(bytes.TrimPrefix(data, utf8BOM), "\n\r"))

	if !bytes.HasPrefix(data[start:], frontmatterDelimiter) {
		return 0, 0, 0, false
	}

	// The opening --- must be followed by a newline (LF or CRLF), not arbitrary text.
	fmStart = start + len(frontmatterDelimiter)
	rest := data[fmStart:]
	if len(rest) == 0 || (rest[0] != '\n' && rest[0] != '\r') {
		return 0, 0, 0, false
	}
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return 0, 0, 0, false
	}

	// The closing --- must be on its own line (followed by newline, CR, or EOF).
	afterClose := rest[idx+len("\n---"):]
	if len(afterClose) > 0 && afterClose[0] != '\n' && afterClose[0] != '\r' {
		return 0, 0, 0, false
	}

	fmEnd = fmStart + idx

	// Body starts after the closing --- and its trailing newline
	bodyStart = fmEnd + len("\n---")
	if bodyStart < len(data) && data[bodyStart] == '\r' {
		bodyStart++
	}
	if bodyStart < len(data) && data[bodyStart] == '\n' {
		bodyStart++
	}

	return fmStart, fmEnd, bodyStart, true
}
