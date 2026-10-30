package item

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// writeMutateFixture writes content to name in a new temp dir and returns its path.
func writeMutateFixture(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	return path
}

// readMutateFile returns the contents of path.
func readMutateFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	return string(data)
}

// Quoted values with escapes end at their real closing quote, so a trailing
// comment is kept whichever path, line pattern or parsed YAML, finds the key.
func TestSetStatus_QuotedValueWithEscapes(t *testing.T) {
	tests := []struct {
		name string
		fm   string
		want string
	}{
		{name: "escaped double quote", fm: "status: \"a\\\"b\" # c\n", want: "status: done # c\n"},
		{name: "doubled single quote", fm: "status: 'it''s' # c\n", want: "status: done # c\n"},
		{name: "escaped key, escaped double quote", fm: "\"st\\x61tus\": \"t\\x6fdo\" # c\n", want: "\"st\\x61tus\": done # c\n"},
		{name: "escaped key, doubled single quote", fm: "\"st\\x61tus\": 'it''s' # c\n", want: "\"st\\x61tus\": done # c\n"},
		{name: "explicit key, escaped double quote", fm: "? status\n: \"a\\\"b\" # c\n", want: "? status\n: done # c\n"},
		{name: "empty double quotes", fm: "status: \"\" # c\n", want: "status: done # c\n"},
		{name: "backslash in single quotes", fm: "status: 'a\\' # c\n", want: "status: done # c\n"},
		{name: "comment marker in single quotes", fm: "status: 'in #progress' # c\n", want: "status: done # c\n"},
		{name: "explicit key, comment marker in double quotes", fm: "? status\n: \"in #progress\" # c\n",
			want: "? status\n: done # c\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMutateFixture(t, "a.md", "---\nid: A\n"+tt.fm+"---\n\nBody.\n")

			if err := SetStatus(path, StatusDone); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			if got, want := readMutateFile(t, path), "---\nid: A\n"+tt.want+"---\n\nBody.\n"; got != want {
				t.Errorf("got:\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// A trailing comment starts at the first # after a space or tab, and the
// whitespace before it is kept.
func TestSetStatus_KeepsTrailingComment(t *testing.T) {
	tests := []struct {
		name string
		fm   string
		want string
	}{
		{name: "tab before comment", fm: "status: todo\t# c\n", want: "status: done\t# c\n"},
		{name: "no space after marker", fm: "status: todo #c\n", want: "status: done #c\n"},
		{name: "marker inside the value", fm: "status: a#b # c\n", want: "status: done # c\n"},
		{name: "two markers", fm: "status: todo # a # b\n", want: "status: done # a # b\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMutateFixture(t, "a.md", "---\nid: A\n"+tt.fm+"---\n\nBody.\n")

			if err := SetStatus(path, StatusDone); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			if got, want := readMutateFile(t, path), "---\nid: A\n"+tt.want+"---\n\nBody.\n"; got != want {
				t.Errorf("got:\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// An empty status key in CRLF frontmatter keeps its CR, whether the value is
// on the next line or missing.
func TestSetStatus_EmptyKeyCRLF(t *testing.T) {
	tests := []struct {
		name string
		fm   string
	}{
		{name: "value on next line", fm: "status:\r\n  todo\r\nid: A\r\n"},
		{name: "no value", fm: "status:\r\nid: A\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMutateFixture(t, "a.md", "---\r\n"+tt.fm+"---\r\nBody.\r\n")

			if err := SetStatus(path, StatusDone); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			if got, want := readMutateFile(t, path), "---\r\nstatus: done\r\nid: A\r\n---\r\nBody.\r\n"; got != want {
				t.Errorf("got:\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// Only keys are matched against the field name: a value that reads "status"
// before an escaped status key is left alone.
func TestSetStatus_ValueNamedStatusIsNotAKey(t *testing.T) {
	path := writeMutateFixture(t, "a.md", "---\nx: status\n\"st\\x61tus\": todo\n---\n\nBody.\n")

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	if got, want := readMutateFile(t, path), "---\nx: status\n\"st\\x61tus\": done\n---\n\nBody.\n"; got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

// An alias equal to the field name is looked up once, so the error names it once.
func TestSetStatusWithOptions_AliasSameAsField(t *testing.T) {
	path := writeMutateFixture(t, "a.md", "---\nid: A\n---\n")

	err := SetStatusWithOptions(path, StatusDone, MutateOptions{Alias: "status"})
	if want := "no status field found in frontmatter of " + path; err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// Values found through the parsed YAML that are not a one-line plain or quoted
// scalar, or whose text on the line does not read back as the value, are
// refused and the file is left alone.
func TestSetStatusWithOptions_RefusesNodeValueForms(t *testing.T) {
	tests := []struct {
		name string
		fm   string
	}{
		{name: "sequence value", fm: "? status\n: [todo]\n"},
		{name: "mapping value", fm: "? status\n: {a: b}\n"},
		{name: "literal value", fm: "? status\n: |\n  todo\n"},
		{name: "folded value", fm: "? status\n: >\n  todo\n"},
		{name: "flow mapping, value then key", fm: "{\"st\\x61tus\": todo, id: A}\n"},
		{name: "flow mapping, empty value at end", fm: "{\"st\\x61tus\":}\n"},
		{name: "flow mapping, empty value before comma", fm: "{\"st\\x61tus\":, id: A}\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := "---\n" + tt.fm + "---\n\nBody.\n"
			path := writeMutateFixture(t, "a.md", orig)

			err := SetStatus(path, StatusDone)
			if err == nil || !strings.Contains(err.Error(), "status key form not supported for writing") {
				t.Errorf("err = %v, want a key form error", err)
			}

			if got := readMutateFile(t, path); got != orig {
				t.Errorf("file changed:\n%q", got)
			}
		})
	}
}

// Frontmatter that parses but is not a mapping is refused, even with a line
// that looks like a status key.
func TestSetStatus_FrontmatterNotAMapping(t *testing.T) {
	for _, fm := range []string{"- status: todo\n", "just text\n"} {
		t.Run(fm, func(t *testing.T) {
			orig := "---\n" + fm + "---\n\nBody.\n"
			path := writeMutateFixture(t, "a.md", orig)

			err := SetStatus(path, StatusDone)
			if err == nil || !strings.Contains(err.Error(), "frontmatter is not a valid YAML mapping") {
				t.Errorf("err = %v, want a not a mapping error", err)
			}

			if got := readMutateFile(t, path); got != orig {
				t.Errorf("file changed:\n%q", got)
			}
		})
	}
}

func TestTopLevelPairs(t *testing.T) {
	tests := []struct {
		name string
		fm   string
		want []string // nil when topLevelPairs returns nil
	}{
		{name: "empty", fm: ""},
		{name: "invalid", fm: "a: [b\n"},
		{name: "sequence", fm: "- a\n- b\n"},
		{name: "scalar", fm: "text\n"},
		{name: "mapping", fm: "id: A\nstatus: todo\n", want: []string{"id", "A", "status", "todo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pairs := topLevelPairs([]byte(tt.fm))
			if tt.want == nil {
				if pairs != nil {
					t.Errorf("got %d nodes, want nil", len(pairs))
				}

				return
			}

			got := make([]string, 0, len(pairs))
			for _, n := range pairs {
				got = append(got, n.Value)
			}

			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeOffset(t *testing.T) {
	src := []byte("ab\r\ncé\nd")

	tests := []struct {
		line, column int
		want         int
		ok           bool
	}{
		{line: 1, column: 1, want: 0, ok: true},
		{line: 1, column: 3, want: 2, ok: true},
		{line: 2, column: 1, want: 4, ok: true},
		{line: 2, column: 3, want: 7, ok: true}, // é is two bytes, one column
		{line: 3, column: 2, want: 9, ok: true},
		{line: 1, column: 4, ok: false}, // past the end of the line
		{line: 2, column: 4, ok: false}, // past the end of an LF line
		{line: 3, column: 3, ok: false}, // past the end of src
		{line: 4, column: 1, ok: false}, // no such line
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d:%d", tt.line, tt.column), func(t *testing.T) {
			got, ok := nodeOffset(src, tt.line, tt.column)
			if ok != tt.ok || (ok && got != tt.want) {
				t.Errorf("nodeOffset = %d, %v, want %d, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// A custom status set replaces the built-in check entirely, and still refuses
// an empty status though StatusSet.Contains accepts one.
func TestSetStatusWithOptions_CustomStatusSet(t *testing.T) {
	const orig = "---\nid: A\nstatus: backlog\n---\n\nBody.\n"

	set := NewStatusSet([]string{"backlog", "review"})

	tests := []struct {
		name    string
		status  Status
		opts    MutateOptions
		wantErr string // "" when the write succeeds
	}{
		{name: "empty", status: "", opts: MutateOptions{ValidStatuses: set}, wantErr: `invalid status ""`},
		{name: "built-in status not in set", status: StatusDone, opts: MutateOptions{ValidStatuses: set},
			wantErr: `invalid status "done"`},
		{name: "custom status without a set", status: "review", wantErr: `invalid status "review": must be todo, in-progress, or done`},
		{name: "custom status in set", status: "review", opts: MutateOptions{ValidStatuses: set}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMutateFixture(t, "a.md", orig)

			err := SetStatusWithOptions(path, tt.status, tt.opts)

			want := orig
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("err = %v, want %q", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Fatalf("SetStatusWithOptions: %v", err)
				}

				want = "---\nid: A\nstatus: review\n---\n\nBody.\n"
			}

			if got := readMutateFile(t, path); got != want {
				t.Errorf("got:\n%q\nwant:\n%q", got, want)
			}
		})
	}
}

// A missing file is reported from resolving the path, before any read.
func TestSetStatus_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")

	err := SetStatus(path, StatusDone)

	// EvalSymlinks reports the resolved path, which differs under a symlinked TMPDIR.
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Op != "lstat" || filepath.Base(pe.Path) != "missing.md" || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want an lstat error for %s", err, path)
	}
}

func TestSetStatus_PathIsADirectory(t *testing.T) {
	dir := t.TempDir()

	err := SetStatus(dir, StatusDone)
	if err == nil || !strings.HasPrefix(err.Error(), "reading "+dir+":") || !errors.Is(err, syscall.EISDIR) {
		t.Errorf("err = %v, want a reading error wrapping EISDIR", err)
	}
}

// The file is replaced by a new inode, so a hard link to it keeps the old
// contents.
func TestSetStatus_HardLinkKeepsOldContents(t *testing.T) {
	path := writeMutateFixture(t, "a.md", testTodoFrontmatter)
	link := filepath.Join(filepath.Dir(path), "link.txt")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err = SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if os.SameFile(before, after) {
		t.Error("path kept its inode, want a new one")
	}

	if got, want := readMutateFile(t, path), "---\nid: TK-001\nstatus: done\n---\n"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}

	if got := readMutateFile(t, link); got != testTodoFrontmatter {
		t.Errorf("link = %q, want the old contents", got)
	}
}

func TestCanWriteInPlace(t *testing.T) {
	yes := []syscall.Errno{syscall.EACCES, syscall.EPERM, syscall.EROFS, syscall.EXDEV, syscall.EBUSY}
	for _, errno := range yes {
		for _, err := range []error{
			errno,
			&fs.PathError{Op: "open", Path: "x", Err: errno},
			&os.LinkError{Op: "rename", Old: "a", New: "b", Err: errno},
		} {
			if !canWriteInPlace(err) {
				t.Errorf("canWriteInPlace(%v) = false, want true", err)
			}
		}
	}

	no := []error{
		syscall.ENOENT,
		syscall.ENAMETOOLONG,
		syscall.ENOSPC,
		&fs.PathError{Op: "open", Path: "x", Err: syscall.EEXIST},
		errors.New("permission denied"),
	}
	for _, err := range no {
		if canWriteInPlace(err) {
			t.Errorf("canWriteInPlace(%v) = true, want false", err)
		}
	}
}

func TestWriteAndClose(t *testing.T) {
	dir := t.TempDir()
	orig := writeMutateFixture(t, "orig.md", "x")
	if err := os.Chmod(orig, 0o640); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(orig)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("writes, sets mode and closes", func(t *testing.T) {
		f, err := os.CreateTemp(dir, "*.tmp")
		if err != nil {
			t.Fatal(err)
		}

		if err = writeAndClose(f, []byte("new"), info); err != nil {
			t.Fatalf("writeAndClose: %v", err)
		}

		if err = f.Close(); !errors.Is(err, os.ErrClosed) {
			t.Errorf("second Close = %v, want ErrClosed", err)
		}

		got, err := os.Stat(f.Name())
		if err != nil {
			t.Fatal(err)
		}

		if got.Mode().Perm() != 0o640 {
			t.Errorf("mode = %v, want 0640", got.Mode().Perm())
		}

		if data := readMutateFile(t, f.Name()); data != "new" {
			t.Errorf("contents = %q, want %q", data, "new")
		}
	})

	t.Run("write error", func(t *testing.T) {
		f, err := os.Open(orig) // read-only, so the write fails
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()

		if err := writeAndClose(f, []byte("new"), info); err == nil || !strings.HasPrefix(err.Error(), "write ") {
			t.Errorf("err = %v, want a write error", err)
		}
	})

	t.Run("chmod error", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("relies on Linux procfs")
		}

		// procfs refuses any mode change, even for root. Writing the thread's
		// own name back to it leaves it unchanged.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		name, err := os.ReadFile("/proc/thread-self/comm")
		if err != nil {
			t.Skip(err)
		}

		f, err := os.OpenFile("/proc/thread-self/comm", os.O_WRONLY, 0)
		if err != nil {
			t.Skip(err)
		}
		defer f.Close()

		if err := writeAndClose(f, name, info); err == nil || !strings.HasPrefix(err.Error(), "chmod ") {
			t.Errorf("err = %v, want a chmod error", err)
		}
	})

	t.Run("sync error", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("relies on Linux pipes taking chmod but not fsync")
		}

		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		defer w.Close()

		// A pipe takes the write and the chmod, but cannot be synced.
		if err := writeAndClose(w, []byte("new"), info); err == nil || !strings.HasPrefix(err.Error(), "sync ") {
			t.Errorf("err = %v, want a sync error", err)
		}
	})
}

// An item whose name is near the 255-byte name limit can still change status:
// the temp file written next to it must not need a longer name.
func TestSetStatus_LongFileName(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "ascii", base: strings.Repeat("a", 247) + ".md"},
		{name: "multi-byte runes", base: strings.Repeat("é", 125) + ".md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMutateFixture(t, tt.base, "---\nstatus: todo\n---\nbody\n")

			if err := SetStatus(path, StatusDone); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			if got, want := readMutateFile(t, path), "---\nstatus: done\n---\nbody\n"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}

			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}

			if len(entries) != 1 {
				t.Errorf("dir holds %d entries, want only the item", len(entries))
			}
		})
	}
}
