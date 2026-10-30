package item

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// MutateOptions controls how SetStatusWithOptions behaves.
type MutateOptions struct {
	// FieldName is the frontmatter key to rewrite (defaults to "status").
	FieldName string
	// Alias is another key the status may be stored under (from field_aliases).
	// The file's own key is rewritten: FieldName when present, as parse reads it
	// first, else Alias.
	Alias string
	// ValidStatuses, when non-nil, replaces the built-in Status.Valid() check.
	ValidStatuses StatusSet
}

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

	if err := checkNewStatus(newStatus, opts.ValidStatuses); err != nil {
		return err
	}

	target, data, err := readItemFile(path)
	if err != nil {
		return err
	}

	fmStart, fmEnd, _, ok := frontmatterBounds(data)
	if !ok {
		return fmt.Errorf("no frontmatter found in %s", path)
	}

	fm := data[fmStart:fmEnd]

	updatedFM, fieldName, err := rewriteStatus(fm, fieldName, opts.Alias, newStatus, path)
	if err != nil {
		return err
	}

	// The rewrite works on lines, so check the YAML still reads back as intended
	// rather than writing a file that no longer loads.
	if !fieldHolds(updatedFM, fieldName, newStatus) {
		return fmt.Errorf("cannot update %s in %s: its value is not a one-line scalar; edit it by hand",
			fieldName, path)
	}

	// Everything outside the frontmatter (BOM, delimiters, line endings, body) is
	// written back byte-for-byte.
	var buf bytes.Buffer
	buf.Write(data[:fmStart])
	buf.Write(updatedFM)
	buf.Write(data[fmEnd:])

	if err := writeFileAtomic(target, buf.Bytes()); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// checkNewStatus returns an error unless newStatus is a status the file may be
// set to: one of valid when it is non-nil, else a built-in status.
func checkNewStatus(newStatus Status, valid StatusSet) error {
	if valid != nil {
		if newStatus == "" || !valid.Contains(newStatus) {
			return fmt.Errorf("invalid status %q", newStatus)
		}
	} else if newStatus == "" || !newStatus.Valid() {
		return fmt.Errorf("invalid status %q: must be todo, in-progress, or done", newStatus)
	}

	return nil
}

// readItemFile returns the file path resolves to and its contents. A symlink is
// resolved once, so the file read is the file written even if the link is
// changed in between.
func readItemFile(path string) (target string, data []byte, err error) {
	target, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", path, err)
	}

	data, err = os.ReadFile(target)
	if err != nil {
		return "", nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return target, data, nil
}

// rewriteStatus returns fm with the status set to newStatus, and the key the
// status is stored under: fieldName when present, as parse reads it first, else
// alias.
func rewriteStatus(fm []byte, fieldName, alias string, newStatus Status, path string) (
	updated []byte, key string, err error,
) {
	keys := []string{fieldName}
	if alias != "" && alias != fieldName {
		keys = append(keys, alias)
	}

	// Without the parsed keys there is no telling the status key's line from a
	// line inside another key's quoted value.
	pairs := topLevelPairs(fm)
	if pairs == nil {
		return nil, "", fmt.Errorf("cannot update %s in %s: frontmatter is not a valid YAML mapping", fieldName, path)
	}

	offsets := keyOffsets(fm, pairs)

	for _, key = range keys {
		out, found, err := rewriteKey(fm, pairs, offsets, key, newStatus, path)
		if err != nil {
			return nil, "", err
		}

		if found {
			return out, key, nil
		}
	}

	if len(keys) == 1 {
		return nil, "", fmt.Errorf("no %s field found in frontmatter of %s", fieldName, path)
	}

	return nil, "", fmt.Errorf("no %s or %s field found in frontmatter of %s", fieldName, alias, path)
}

// keyOffsets returns the byte offset in fm of each key in pairs, in turn, or
// -1 where it has none.
func keyOffsets(fm []byte, pairs []*yaml.Node) []int {
	offsets := make([]int, 0, len(pairs)/2)
	// Keys come in document order, so each line scan resumes where the last ended.
	lineStart, line := 0, 1

	for i := 0; i+1 < len(pairs); i += 2 {
		key := pairs[i]
		off := -1

		if key.Line < line {
			if o, ok := nodeOffset(fm, key.Line, key.Column); ok {
				off = o
			}
		} else if start, ok := advanceLines(fm, lineStart, key.Line-line); ok {
			lineStart, line = start, key.Line
			if o, ok := advanceColumns(fm, start, key.Column-1); ok {
				off = o
			}
		}

		offsets = append(offsets, off)
	}

	return offsets
}

// rewriteKey returns fm with the status stored under key set to newStatus, and
// reports whether fm has that key at all. offsets holds the offset of each key
// in pairs, as keyOffsets returns them.
func rewriteKey(fm []byte, pairs []*yaml.Node, offsets []int, key string, newStatus Status, path string) (
	updated []byte, found bool, err error,
) {
	re := statusFieldRe(key)
	var matches [][]int

	first := -1

	for i := 0; i+1 < len(pairs); i += 2 {
		if first < 0 && keyName(pairs[i]) == key {
			first = i
		}

		if m := keyLineMatch(fm, re, pairs[i], offsets[i/2], key); m != nil {
			matches = append(matches, m)
		}
	}

	if len(matches) > 0 {
		return replaceMatches(fm, matches, newStatus), true, nil
	}

	if first < 0 {
		return nil, false, nil
	}

	// A key the line pattern misses, such as "st\x61tus" or "? status", still
	// loads, so find its value through the parsed YAML. An alias key (*s) is
	// refused: its text is not the key it reads as.
	if pairs[first].Kind == yaml.ScalarNode {
		if out, ok := replaceNodeValue(fm, pairs[first+1], newStatus); ok {
			return out, true, nil
		}
	}

	return nil, true, fmt.Errorf("cannot update %s in %s: status key form not supported for writing; "+
		"use a plain %s: key", key, path, key)
}

// keyLineMatch returns the submatch indexes, in fm, of the re match at the
// start of the line holding k, a top-level key at offset off, or nil unless k
// is a scalar key named key and its line matches. A line inside another key's
// multi-line quoted value can also match re, and must not be rewritten.
func keyLineMatch(fm []byte, re *regexp.Regexp, k *yaml.Node, off int, key string) []int {
	if k.Kind != yaml.ScalarNode || k.Value != key || off < 0 || (off > 0 && fm[off-1] != '\n') {
		return nil
	}

	m := re.FindSubmatchIndex(fm[off:])
	for i := range m {
		m[i] += off
	}

	return m
}

// replaceMatches returns fm with each status match in matches rewritten to newStatus.
func replaceMatches(fm []byte, matches [][]int, newStatus Status) []byte {
	out := make([]byte, 0, len(fm))
	prev := 0

	for _, m := range matches {
		out = append(out, fm[prev:m[0]]...)
		out = append(out, replaceStatusValue(fm[m[0]:m[1]], m[3]-m[0], newStatus)...)
		prev = m[1]
	}

	return append(out, fm[prev:]...)
}

// topLevelPairs returns the keys and values of the mapping fm holds, in turn, or
// nil when fm is not a YAML mapping.
func topLevelPairs(fm []byte) []*yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}

	return doc.Content[0].Content
}

// replaceNodeValue returns fm with the scalar value, found by its line and
// column, replaced by newStatus, keeping everything around it. It reports false
// unless value is a plain or quoted scalar on one line whose source text it
// finds at that position, or an empty value right after its key's colon.
func replaceNodeValue(fm []byte, value *yaml.Node, newStatus Status) ([]byte, bool) {
	if !plainOrQuoted(value) {
		return nil, false
	}

	start, ok := nodeOffset(fm, value.Line, value.Column)
	if !ok {
		return nil, false
	}

	line := fm[start:]
	if i := bytes.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}

	if value.Value == "" && value.Style == 0 {
		return fillEmptyValue(fm, start, line, newStatus)
	}

	end := plainEnd(line)
	if value.Style != 0 {
		end = quotedEnd(line)
	}

	// The text must read back as the whole value: a scalar that goes on past
	// this line, or a position that does not point at it, does not.
	if !readsAs(line[:end], value.Value) {
		return nil, false
	}

	out := append(bytes.Clone(fm[:start]), newStatus...)

	return append(out, fm[start+end:]...), true
}

// plainOrQuoted reports whether value is a plain or quoted scalar with no
// anchor: not tagged, literal or folded.
func plainOrQuoted(value *yaml.Node) bool {
	return value.Kind == yaml.ScalarNode && value.Anchor == "" &&
		(value.Style == 0 || value.Style == yaml.DoubleQuotedStyle || value.Style == yaml.SingleQuotedStyle)
}

// readsAs reports whether text parses as a YAML scalar whose value is want.
func readsAs(text []byte, want string) bool {
	var got yaml.Node
	if err := yaml.Unmarshal(text, &got); err != nil || len(got.Content) == 0 {
		return false
	}

	return got.Content[0].Kind == yaml.ScalarNode && got.Content[0].Value == want
}

// fillEmptyValue returns fm with newStatus written after the colon that ends at
// start, as replaceStatusValue writes a plain "status:" key with no value. line
// is the rest of that line, which must hold only spaces or a comment. It
// reports false when start does not follow a colon (a "? status" key with no
// ": " line, or a flow mapping).
func fillEmptyValue(fm []byte, start int, line []byte, newStatus Status) ([]byte, bool) {
	if start == 0 || fm[start-1] != ':' {
		return nil, false
	}

	hasValue, tail := splitScalar(line)
	if hasValue {
		return nil, false
	}

	out := append(bytes.Clone(fm[:start]), " "+string(newStatus)...)
	out = append(out, tail...)

	return append(out, fm[start+len(line):]...), true
}

// nodeOffset returns the byte offset in src of a yaml.Node's 1-based line and
// column, counting line breaks and characters as the YAML parser does.
func nodeOffset(src []byte, line, column int) (int, bool) {
	off, ok := advanceLines(src, 0, line-1)
	if !ok {
		return 0, false
	}

	return advanceColumns(src, off, column-1)
}

// advanceLines returns the offset reached from off after skipping lines line
// breaks.
func advanceLines(src []byte, off, lines int) (int, bool) {
	for ; lines > 0; lines-- {
		for {
			if off >= len(src) {
				return 0, false
			}

			if n := lineBreakLen(src[off:]); n > 0 {
				off += n

				break
			}

			_, size := utf8.DecodeRune(src[off:])
			off += size
		}
	}

	return off, true
}

// advanceColumns returns the offset reached from off after skipping columns
// characters, none of them a line break.
func advanceColumns(src []byte, off, columns int) (int, bool) {
	for ; columns > 0; columns-- {
		if off >= len(src) || lineBreakLen(src[off:]) > 0 {
			return 0, false
		}

		_, size := utf8.DecodeRune(src[off:])
		off += size
	}

	return off, true
}

// lineBreakLen returns the length of the line break at the start of b, or 0.
// Like the YAML parser, it takes CRLF, CR, LF, NEL, LS and PS as line breaks.
func lineBreakLen(b []byte) int {
	for _, br := range []string{"\r\n", "\r", "\n", "\u0085", " ", " "} {
		if bytes.HasPrefix(b, []byte(br)) {
			return len(br)
		}
	}

	return 0
}

// statusFieldRe matches a fieldName key line at the start of the text through
// its end, plus an indented value on the next line (YAML allows
// "status:\n  todo"). The key may be quoted or followed by spaces
// ("status" : todo); group 1 is the key through its colon. replaceStatusValue
// decides what to keep.
func statusFieldRe(fieldName string) *regexp.Regexp {
	key := regexp.QuoteMeta(fieldName)
	single := regexp.QuoteMeta(strings.ReplaceAll(fieldName, "'", "''"))

	return regexp.MustCompile(`^((?:` + key + `|"` + key + `"|'` + single + `')[ \t]*:)` +
		`[^\r\n]*(?:\r?\n[ \t]+[^\s#][^\r\n]*)?`)
}

// createTemp and renameFile are os.CreateTemp and os.Rename, swapped in tests to
// make them fail on demand.
var (
	createTemp = os.CreateTemp
	renameFile = os.Rename
)

// writeFileAtomic replaces the file at path with data, so a crash or a full disk
// leaves either the old or the new contents, never a truncated file. It writes a
// temp file next to the file, syncs it, gives it the file's mode and, where it
// can, its owner, and renames it over the file. path must not be a symlink.
// Hard links to the file are not kept: the path gets a new inode.
//
// Where the temp file cannot be created or renamed over the file (a read-only
// directory, a bind-mounted file), it falls back to writing the file in place.
func writeFileAtomic(path string, data []byte) error {
	info, err := writableInfo(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	// The name ends in .tmp, not .md, so LoadAll never picks it up. It leaves out
	// the item's name, so it fits wherever that name does.
	tmp, err := createTemp(dir, ".yat-*.tmp")
	if err != nil {
		if canWriteInPlace(err) {
			return os.WriteFile(path, data, info.Mode().Perm())
		}

		return err
	}

	if err := writeAndClose(tmp, data, info); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())

		return err
	}

	if err := renameFile(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())

		if canWriteInPlace(err) {
			return os.WriteFile(path, data, info.Mode().Perm())
		}

		return err
	}

	syncDir(dir)

	return nil
}

// writableInfo returns the file info of the file at path, refusing a file the
// user cannot write: renaming over it needs only a writable directory, but
// writing it in place would fail.
func writableInfo(path string) (fs.FileInfo, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return f.Stat()
}

// canWriteInPlace reports whether err, from creating the temp file or renaming
// it, is one where writing the file in place may still work.
func canWriteInPlace(err error) bool {
	for _, errno := range []syscall.Errno{syscall.EACCES, syscall.EPERM, syscall.EROFS, syscall.EXDEV, syscall.EBUSY} {
		if errors.Is(err, errno) {
			return true
		}
	}

	return false
}

// syncDir flushes the directory so the rename survives a crash. It is best
// effort: the file has already been replaced, and some filesystems do not
// support syncing a directory.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}

	_ = d.Sync() //nolint:errcheck // best effort: the file is already replaced
	_ = d.Close()
}

// writeAndClose writes data to f, gives it orig's owner (best effort) and mode,
// syncs it to disk and closes it.
func writeAndClose(f *os.File, data []byte, orig fs.FileInfo) error {
	if _, err := f.Write(data); err != nil {
		return err
	}

	keepOwner(f, orig)

	if err := f.Chmod(orig.Mode().Perm()); err != nil {
		return err
	}

	if err := f.Sync(); err != nil {
		return err
	}

	return f.Close()
}

// replaceStatusValue rewrites one "field: value" match, keeping the key as
// written and any trailing comment. m is the key line, optionally followed by an
// indented next line, and m[:keyEnd] is the key through its colon.
func replaceStatusValue(m []byte, keyEnd int, newStatus Status) []byte {
	rest := m[keyEnd:]
	line, next, hasNext := bytes.Cut(rest, []byte("\n"))
	out := append(bytes.Clone(m[:keyEnd]), " "+string(newStatus)...)

	hasValue, tail := splitScalar(line)
	if hasValue || !hasNext {
		out = append(out, tail...)
		if hasNext {
			// The value is on the key line, so the next line is not part of it.
			out = append(out, rest[len(line):]...)
		}

		return out
	}

	// The value is on the next line; fold it onto the key line, keeping a
	// comment from the key line.
	if bytes.HasPrefix(tail, []byte(" #")) {
		out = append(out, bytes.TrimRight(tail, "\r")...)
	}

	_, nextTail := splitScalar(next)

	return append(out, nextTail...)
}

// fieldHolds reports whether fm parses as a YAML mapping whose fieldName key
// holds exactly the scalar want.
func fieldHolds(fm []byte, fieldName string, want Status) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false
	}

	found := false
	pairs := doc.Content[0].Content

	for i := 0; i+1 < len(pairs); i += 2 {
		if keyName(pairs[i]) != fieldName {
			continue
		}

		if v := pairs[i+1]; v.Kind != yaml.ScalarNode || v.Value != string(want) {
			return false
		}

		found = true
	}

	return found
}

// splitScalar splits the text after a YAML key's colon into the scalar value
// and what follows it (a comment, trailing whitespace or a CR), and reports
// whether there was a value at all.
func splitScalar(s []byte) (hasValue bool, tail []byte) {
	r := bytes.TrimLeft(s, " \t")

	switch {
	case len(r) == 0 || r[0] == '\r':
		return false, r
	case r[0] == '#':
		return false, append([]byte(" "), r...)
	}

	var end int

	switch r[0] {
	case '"', '\'':
		end = quotedEnd(r)
	default:
		end = plainEnd(r)
	}

	return true, r[end:]
}

// quotedEnd returns the index just past the closing quote of the quoted scalar
// at the start of r, or len(r) if it is not closed on this line.
func quotedEnd(r []byte) int {
	quote := r[0]

	for i := 1; i < len(r); i++ {
		switch {
		case quote == '"' && r[i] == '\\':
			i++ // skip the escaped character
		case quote == '\'' && r[i] == '\'' && i+1 < len(r) && r[i+1] == '\'':
			i++ // '' is an escaped single quote
		case r[i] == quote:
			return i + 1
		}
	}

	return len(r)
}

// plainEnd returns the length of the plain scalar at the start of r, which ends
// at " #" (a comment) or the end of the line, without trailing whitespace.
func plainEnd(r []byte) int {
	end := len(r)

	for i := 1; i < len(r); i++ {
		if r[i] == '#' && (r[i-1] == ' ' || r[i-1] == '\t') {
			end = i

			break
		}
	}

	return len(bytes.TrimRight(r[:end], " \t\r"))
}
