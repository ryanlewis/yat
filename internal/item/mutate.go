package item

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

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

	fieldName, fieldRe, err := statusKey(fm, fieldName, opts.Alias, path)
	if err != nil {
		return err
	}

	updatedFM := fieldRe.ReplaceAllFunc(fm, func(m []byte) []byte {
		return replaceStatusValue(m, fieldName, newStatus)
	})

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

// statusKey returns the key the status is stored under in fm, and the pattern
// matching its line: fieldName when present, as parse reads it first, else alias.
func statusKey(fm []byte, fieldName, alias, path string) (string, *regexp.Regexp, error) {
	if re := statusFieldRe(fieldName); re.Match(fm) {
		return fieldName, re, nil
	}

	if alias == "" || alias == fieldName {
		return "", nil, fmt.Errorf("no %s field found in frontmatter of %s", fieldName, path)
	}

	if re := statusFieldRe(alias); re.Match(fm) {
		return alias, re, nil
	}

	return "", nil, fmt.Errorf("no %s or %s field found in frontmatter of %s", fieldName, alias, path)
}

// statusFieldRe matches a fieldName key line through its end, plus an indented
// value on the next line (YAML allows "status:\n  todo"). replaceStatusValue
// decides what to keep.
func statusFieldRe(fieldName string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(fieldName) + `:[^\r\n]*(?:\r?\n[ \t]+[^\s#][^\r\n]*)?`)
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

// replaceStatusValue rewrites one "field: value" match, keeping any trailing
// comment. m is the key line, optionally followed by an indented next line.
func replaceStatusValue(m []byte, fieldName string, newStatus Status) []byte {
	rest := m[len(fieldName)+1:]
	line, next, hasNext := bytes.Cut(rest, []byte("\n"))
	out := []byte(fieldName + ": " + string(newStatus))

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
