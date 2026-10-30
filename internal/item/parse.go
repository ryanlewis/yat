package item

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"slices"
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

// statusField is the canonical frontmatter key for an item's status.
const statusField = "status"

// ErrMissingID is returned when a file has item-like frontmatter but no id field.
var ErrMissingID = errors.New("item-like frontmatter has no id field")

// ErrUnclosedFrontmatter is returned when a file opens frontmatter with a ---
// line but no later line closes it.
var ErrUnclosedFrontmatter = errors.New("frontmatter has no closing --- line")

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
// Returns ErrUnclosedFrontmatter (wrapped) if the frontmatter never closes.
// Returns an error if status or priority values are not recognized.
func Parse(data []byte, path string) (*Item, error) {
	return ParseWithOptions(data, path, ParseOptions{})
}

// ParseWithOptions is like Parse but accepts custom parsing options for
// field aliasing and custom status validation.
func ParseWithOptions(data []byte, path string, opts ParseOptions) (*Item, error) {
	fm, body, ok := splitFrontmatter(data)
	if !ok {
		return nil, unclosedError(data, path)
	}

	if hasDirective(fm, "ignore") {
		return nil, nil
	}

	fm, statusKey, err := remapAliases(fm, opts.Aliases, path)
	if err != nil {
		return nil, err
	}

	item, err := decodeItem(fm, path)
	if err != nil {
		return nil, err
	}

	if item.ID == "" {
		if looksLikeItem(item) {
			return nil, fmt.Errorf("%w: %s", ErrMissingID, path)
		}

		return nil, nil
	}

	if err := validateFields(item, opts, path); err != nil {
		return nil, err
	}

	item.Dependencies = dedupe(item.Dependencies)
	item.FilePath = path
	item.StatusKey = statusKey
	item.Body = string(body)

	if item.Title == "" {
		item.Title = extractHeading(body)
	}

	return item, nil
}

// decodeItem decodes frontmatter into an Item, leaving the yat directive out of
// its extra fields.
func decodeItem(fm []byte, path string) (*Item, error) {
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

	return &item, nil
}

// dedupe returns ids with repeats removed, keeping the first occurrence of each.
func dedupe(ids []string) []string {
	if len(ids) < 2 {
		return ids
	}

	seen := make(map[string]bool, len(ids))
	out := ids[:0]

	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}

	return out
}

// extractHeading returns the text of the first ATX heading (h1–h3) in the body, or "".
// Headings inside fenced code blocks (``` or ~~~) and indented code are ignored.
// Indent is measured from the content column of any list item the line sits
// under, so a heading indented under "- step" still counts.
func extractHeading(body []byte) string {
	var s headingScanner

	for _, line := range bytes.Split(body, []byte("\n")) {
		if title, ok := s.scan(line); ok {
			return title
		}
	}

	return ""
}

// headingScanner tracks the block context extractHeading needs between lines:
// open fences, open list items and whether a paragraph is running.
type headingScanner struct {
	fence      []byte // marker of the open fence, nil outside one
	fenceDepth int    // list items open when the fence opened
	lists      []int  // content columns of the open list items, innermost last
	para       bool   // the previous line was paragraph text
}

// scan reads one line and returns the heading text when the line is a heading.
func (s *headingScanner) scan(line []byte) (string, bool) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		s.para = false
		return "", false
	}

	start := indentWidth(line)
	inList := len(s.lists) > 0

	if s.lazyContinues(trimmed, start) {
		return "", false
	}

	// Four columns of indent make a code line: never a heading or a fence.
	if s.closeBlocks(start) >= codeIndent {
		return "", false
	}

	if s.scanFence(trimmed) || s.scanListItem(trimmed, start, inList) {
		return "", false
	}

	if title, ok := atxHeading(trimmed); ok {
		return title, true
	}

	s.para = true

	return "", false
}

// lazyContinues reports whether line, at start columns, lazily continues the
// paragraph of the innermost list item. A line indented less than an item's
// content is outside that item, unless it continues the item's paragraph.
func (s *headingScanner) lazyContinues(line []byte, start int) bool {
	return s.para && len(s.lists) > 0 && start < s.lists[len(s.lists)-1] &&
		!interruptsParagraph(line, start, true)
}

// closeBlocks closes the list items, and any fence inside them, that a line at
// start columns falls outside, and returns the line's indent from the content
// column of the innermost item still open.
func (s *headingScanner) closeBlocks(start int) int {
	var indent int
	s.lists, indent = listIndent(s.lists, start)

	if s.fence != nil && len(s.lists) < s.fenceDepth {
		s.fence = nil // closing the item closes the fence inside it
	}

	return indent
}

// scanListItem opens a list item when line, at start columns, starts one, and
// reports whether it did.
func (s *headingScanner) scanListItem(line []byte, start int, inList bool) bool {
	col := listContentColumn(line, start)
	if col == 0 || (s.para && !interruptsParagraph(line, start, inList)) {
		return false
	}

	s.lists = append(s.lists, col)
	s.para = len(line) > listMarker(line)

	return true
}

// scanFence opens or closes a fence on line and reports whether line is fenced
// code or a fence marker.
func (s *headingScanner) scanFence(line []byte) bool {
	if s.fence != nil {
		// Only a bare run of the same character, at least as long, closes the fence.
		if run := fenceMarker(line); run != nil && len(run) == len(line) &&
			run[0] == s.fence[0] && len(run) >= len(s.fence) {
			s.fence = nil
		}

		return true
	}

	if s.fence = fenceMarker(line); s.fence != nil {
		s.fenceDepth, s.para = len(s.lists), false
		return true
	}

	return false
}

// interruptsParagraph reports whether line starts a heading, fence or list item
// rather than continuing the paragraph above it. Outside a list, only a list
// item that starts at 1 and is not empty can interrupt a paragraph.
func interruptsParagraph(line []byte, start int, inList bool) bool {
	if _, ok := atxHeading(line); ok || fenceMarker(line) != nil {
		return true
	}

	n := listMarker(line)
	if n == 0 || listContentColumn(line, start) == 0 {
		return false
	}

	return inList || (len(line) > n && (n == 1 || string(line[:n-1]) == "1"))
}

// atxHeading returns the text of line when it is an h1–h3 ATX heading.
func atxHeading(line []byte) (string, bool) {
	for _, prefix := range []string{"# ", "## ", "### "} {
		if bytes.HasPrefix(line, []byte(prefix)) {
			return string(bytes.TrimSpace(line[len(prefix):])), true
		}
	}

	return "", false
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

	// A backtick fence's info string cannot hold a backtick: ```a`b is inline code.
	if line[0] == '`' && bytes.IndexByte(line[n:], '`') >= 0 {
		return nil
	}

	return line[:n]
}

// listIndent closes the list items in lists that a non-blank line at indent
// columns falls outside, and returns the rest and the line's indent from the
// content column of the innermost item still open.
func listIndent(lists []int, indent int) (open []int, rel int) {
	for len(lists) > 0 && indent < lists[len(lists)-1] {
		lists = lists[:len(lists)-1]
	}

	if len(lists) > 0 {
		indent -= lists[len(lists)-1]
	}

	return lists, indent
}

// maxOrderedDigits is the most digits an ordered list marker can have.
const maxOrderedDigits = 9

// listMarker returns the length of the list marker (-, *, + or 1. / 1)) that
// line starts with, or 0.
func listMarker(line []byte) int {
	if len(line) > 0 && isBullet(line[0]) {
		return 1
	}

	n := 0
	for n < min(len(line), maxOrderedDigits) && isDigit(line[n]) {
		n++
	}

	if n > 0 && n < len(line) && (line[n] == '.' || line[n] == ')') {
		return n + 1
	}

	return 0
}

// isBullet reports whether c marks a bullet list item.
func isBullet(c byte) bool {
	return c == '-' || c == '*' || c == '+'
}

// isDigit reports whether c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// listContentColumn returns the column a list item's content starts at, for
// a line trimmed of the start indent columns, or 0 when it does not start one.
func listContentColumn(line []byte, start int) int {
	n := listMarker(line)
	if n == 0 || isThematicBreak(line) {
		return 0
	}

	if n == len(line) {
		return start + n + 1 // an empty item: its content starts one column on
	}

	if line[n] != ' ' && line[n] != '\t' {
		return 0
	}

	return markerGapEnd(line, start, n)
}

// markerGapEnd returns the column a list item's content starts at, given the
// marker is the first n bytes of line and is followed by a space or tab.
func markerGapEnd(line []byte, start, n int) int {
	// One to four columns of space follow the marker; with five or more, the
	// content starts after one and the rest is indented code.
	col := start + n
	for _, c := range line[n:] {
		switch c {
		case ' ':
			col++
		case '\t':
			col += codeIndent - col%codeIndent
		default:
			if gap := col - start - n; gap > codeIndent {
				return start + n + 1
			}

			return col
		}
	}

	return col
}

// minBreakLen is the fewest marks that make a thematic break.
const minBreakLen = 3

// isThematicBreak reports whether line is a thematic break such as "- - -" or
// "***", which looks like a list item but is not one.
func isThematicBreak(line []byte) bool {
	n := 0

	for _, c := range line {
		switch {
		case c == line[0]:
			n++
		case c != ' ' && c != '\t':
			return false
		}
	}

	return (line[0] == '-' || line[0] == '*' || line[0] == '_') && n >= minBreakLen
}

// codeIndent is the indent, in columns, at which a line becomes indented code.
const codeIndent = 4

// indentWidth returns the columns of leading whitespace in line, with tabs
// advancing to the next multiple of four.
func indentWidth(line []byte) int {
	w := 0

	for _, c := range line {
		switch c {
		case ' ':
			w++
		case '\t':
			w += codeIndent - w%codeIndent
		default:
			return w
		}
	}

	return w
}

// remapAliases rewrites aliased frontmatter keys to their canonical names, and
// returns the key the file stores the status under ("" when it has none).
//
// A key that is an alias always means the field it is an alias for, never a
// field of its own name (status: state with priority: status reads a status key
// as the priority). Every rename is decided from the file's own keys, so the
// result does not depend on map order.
func remapAliases(fm []byte, aliases map[string]string, path string) (remapped []byte, from string, err error) {
	// Rename keys in the node tree rather than round-tripping through a map, so
	// values keep their source text (007, 1.10) and key order and anchors survive.
	var doc yaml.Node
	if err = yaml.Unmarshal(fm, &doc); err != nil {
		if len(aliases) == 0 {
			return fm, "", nil // decoding the item reports it
		}

		return nil, "", fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fm, "", nil
	}

	from = renameAliasKeys(doc.Content[0], aliases)
	if len(aliases) == 0 {
		return fm, from, nil
	}

	remapped, err = yaml.Marshal(&doc)
	if err != nil {
		return nil, "", fmt.Errorf("re-marshaling frontmatter in %s: %w", path, err)
	}

	return remapped, from, nil
}

// renameAliasKeys renames the alias keys in mapping, as remapAliases describes,
// and returns the key the status is stored under ("" when there is none).
func renameAliasKeys(mapping *yaml.Node, aliases map[string]string) (from string) {
	isAlias := func(key string) bool { return aliases[key] != "" && aliases[key] != key }
	owned := ownedFields(mapping, isAlias)
	targets := aliasTargets(aliases)

	if owned[statusField] {
		from = statusField
	}

	content := make([]*yaml.Node, 0, len(mapping.Content))

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		name := keyName(key)

		switch canonical := aliases[name]; {
		case !isAlias(name):
		case !owned[canonical]:
			if canonical == statusField {
				from = name
			}

			key = renameKey(key, canonical)
		case targets[name]:
			// The field's own key wins. This alias key would read as the field
			// it is named after, which another alias stands for, so drop it.
			continue
		default:
			// The field's own key wins; the alias key stays an extra field.
		}

		content = append(content, key, value)
	}

	mapping.Content = content

	return from
}

// ownedFields returns the fields the file has a key of their own for: the keys
// of mapping that isAlias does not report as aliases.
func ownedFields(mapping *yaml.Node, isAlias func(string) bool) map[string]bool {
	owned := make(map[string]bool, len(mapping.Content)/2)

	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if key := keyName(mapping.Content[i]); !isAlias(key) {
			owned[key] = true
		}
	}

	return owned
}

// aliasTargets returns the fields some alias stands for.
func aliasTargets(aliases map[string]string) map[string]bool {
	targets := make(map[string]bool, len(aliases))

	for alias, canonical := range aliases {
		if alias != canonical {
			targets[canonical] = true
		}
	}

	return targets
}

// renameKey renames the mapping key node key to name and returns it.
func renameKey(key *yaml.Node, name string) *yaml.Node {
	if key.Kind == yaml.AliasNode {
		// Renaming an alias would point it at another anchor.
		key = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str"}
	}

	key.Value = name

	return key
}

// keyName returns the key a mapping key node reads as. An alias used as a key
// (*s) reads as its anchor's value, as decoding takes it, not the anchor's name.
func keyName(key *yaml.Node) string {
	if key.Kind == yaml.AliasNode && key.Alias != nil {
		return key.Alias.Value
	}

	return key.Value
}

// validateFields checks the status and priority fields on an item, and
// normalizes the status.
func validateFields(item *Item, opts ParseOptions, path string) error {
	if err := validateStatus(item, opts, path); err != nil {
		return err
	}

	if !item.Priority.Valid() {
		return fmt.Errorf("invalid priority %q in %s: must be Critical, High, Medium, or Low", item.Priority, path)
	}

	return nil
}

// validateStatus checks and normalizes the status field on an item.
func validateStatus(item *Item, opts ParseOptions, path string) error {
	if opts.ValidStatuses != nil {
		if !opts.ValidStatuses.Contains(item.Status) {
			valid := make([]string, 0, len(opts.ValidStatuses))
			for s := range opts.ValidStatuses {
				valid = append(valid, string(s))
			}

			slices.Sort(valid)

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
	fmStart, ok = openingEnd(data)
	if !ok {
		return 0, 0, 0, false
	}

	idx, closeLen := closingLine(data[fmStart:])
	if idx < 0 {
		return 0, 0, 0, false
	}

	fmEnd = fmStart + idx

	// Body starts after the closing --- and its trailing newline
	bodyStart = skipByte(data, fmEnd+closeLen, '\r')
	bodyStart = skipByte(data, bodyStart, '\n')

	return fmStart, fmEnd, bodyStart, true
}

// unclosedError returns ErrUnclosedFrontmatter (wrapped) when data has no
// frontmatter but opens with a --- line, and nil when it does not open with one.
func unclosedError(data []byte, path string) error {
	if _, ok := openingEnd(data); ok {
		return fmt.Errorf("%w: %s", ErrUnclosedFrontmatter, path)
	}

	return nil
}

// openingEnd returns the index just past the opening --- in data, and whether
// data opens with one. A leading UTF-8 BOM and blank lines are skipped.
func openingEnd(data []byte) (int, bool) {
	start := len(data) - len(bytes.TrimLeft(bytes.TrimPrefix(data, utf8BOM), "\n\r"))

	if !bytes.HasPrefix(data[start:], frontmatterDelimiter) {
		return 0, false
	}

	// The opening --- must be followed by a newline (LF or CRLF), not arbitrary text.
	end := start + len(frontmatterDelimiter)
	if end == len(data) || !isLineBreak(data[end]) {
		return 0, false
	}

	return end, true
}

// closingLine finds the line that closes the frontmatter in rest: --- on its
// own line, optionally followed by spaces or tabs. It returns the index of the
// newline before that line and the line's length counting that newline, or -1.
func closingLine(rest []byte) (idx, n int) {
	for off := 0; ; {
		i := bytes.Index(rest[off:], []byte("\n---"))
		if i < 0 {
			return -1, 0
		}

		idx = off + i
		end := idx + len("\n---")

		for end < len(rest) && (rest[end] == ' ' || rest[end] == '\t') {
			end++
		}

		if end == len(rest) || isLineBreak(rest[end]) {
			return idx, end - idx
		}

		off = idx + 1
	}
}

// isLineBreak reports whether c is a line feed or carriage return.
func isLineBreak(c byte) bool {
	return c == '\n' || c == '\r'
}

// skipByte returns i+1 when data[i] is c, else i.
func skipByte(data []byte, i int, c byte) int {
	if i < len(data) && data[i] == c {
		return i + 1
	}

	return i
}
