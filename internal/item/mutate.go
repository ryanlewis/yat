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

	if opts.ValidStatuses != nil {
		if newStatus == "" || !opts.ValidStatuses.Contains(newStatus) {
			return fmt.Errorf("invalid status %q", newStatus)
		}
	} else if newStatus == "" || !newStatus.Valid() {
		return fmt.Errorf("invalid status %q: must be todo, in-progress, or done", newStatus)
	}

	// Resolve a symlink once, so the file read is the file written even if the
	// link is changed in between.
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
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

	pairs := topLevelPairs(fm)

	for _, key = range keys {
		if re := statusFieldRe(key); re.Match(fm) {
			return re.ReplaceAllFunc(fm, func(m []byte) []byte {
				return replaceStatusValue(m, re.FindSubmatchIndex(m)[3], newStatus)
			}), key, nil
		}

		// A key the line pattern misses, such as "st\x61tus" or "? status",
		// still loads, so find its value through the parsed YAML.
		for i := 0; i+1 < len(pairs); i += 2 {
			if keyName(pairs[i]) != key {
				continue
			}

			// An alias key (*s) is refused: its text is not the key it reads as.
			if pairs[i].Kind == yaml.ScalarNode {
				if out, ok := replaceNodeValue(fm, pairs[i+1], newStatus); ok {
					return out, key, nil
				}
			}

			return nil, "", fmt.Errorf("cannot update %s in %s: status key form not supported for writing; "+
				"use a plain %s: key", key, path, key)
		}
	}

	if len(keys) == 1 {
		return nil, "", fmt.Errorf("no %s field found in frontmatter of %s", fieldName, path)
	}

	return nil, "", fmt.Errorf("no %s or %s field found in frontmatter of %s", fieldName, alias, path)
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
	if value.Kind != yaml.ScalarNode || value.Anchor != "" {
		return nil, false
	}

	if value.Style != 0 && value.Style != yaml.DoubleQuotedStyle && value.Style != yaml.SingleQuotedStyle {
		return nil, false // tagged, literal or folded
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
	var got yaml.Node
	if err := yaml.Unmarshal(line[:end], &got); err != nil || len(got.Content) == 0 ||
		got.Content[0].Kind != yaml.ScalarNode || got.Content[0].Value != value.Value {
		return nil, false
	}

	out := append(bytes.Clone(fm[:start]), newStatus...)

	return append(out, fm[start+end:]...), true
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
	off := 0

	for ; line > 1; line-- {
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

	for ; column > 1; column-- {
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

// statusFieldRe matches a fieldName key line through its end, plus an indented
// value on the next line (YAML allows "status:\n  todo"). The key may be quoted
// or followed by spaces ("status" : todo); group 1 is the key through its colon.
// replaceStatusValue decides what to keep.
func statusFieldRe(fieldName string) *regexp.Regexp {
	key := regexp.QuoteMeta(fieldName)
	single := regexp.QuoteMeta(strings.ReplaceAll(fieldName, "'", "''"))

	return regexp.MustCompile(`(?m)^((?:` + key + `|"` + key + `"|'` + single + `')[ \t]*:)` +
		`[^\r\n]*(?:\r?\n[ \t]+[^\s#][^\r\n]*)?`)
}

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

	// The name ends in .tmp, not .md, so LoadAll never picks it up.
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
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

	if err := os.Rename(tmp.Name(), path); err != nil {
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
		if pairs[i].Value != fieldName {
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
