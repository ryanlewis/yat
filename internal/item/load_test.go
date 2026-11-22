package item

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func testdataDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file path")
	}

	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

func TestLoadAll(t *testing.T) {
	dir := testdataDir(t)

	items, _, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	// Should find SP-001, TK-001, TK-002, TK-003, ST-001, ST-002
	// SPEC.md should be skipped (no id field)
	if len(items) != 6 {
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		t.Fatalf("expected 6 items, got %d: %v", len(items), ids)
	}
}

func TestLoadAllIndex(t *testing.T) {
	dir := testdataDir(t)

	items, _, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	idx, err := Index(items)
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if _, ok := idx["TK-001"]; !ok {
		t.Error("TK-001 not found in index")
	}

	if _, ok := idx["SP-001"]; !ok {
		t.Error("SP-001 not found in index")
	}

	// SPEC.md should not be in the index
	for id := range idx {
		if id == "" {
			t.Error("empty ID found in index")
		}
	}
}

// loadWithWarnings runs LoadAll on dir and returns the items and the warning text.
func loadWithWarnings(t *testing.T, dir string) ([]*Item, string, error) {
	t.Helper()

	var buf bytes.Buffer

	old := warnOut
	warnOut = &buf

	t.Cleanup(func() { warnOut = old })

	items, _, err := LoadAll(dir)

	return items, buf.String(), err
}

func TestLoadAll_ReturnsSkippedPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\nid: A-1\n---\n")
	writeFile(t, filepath.Join(dir, "list.md"), "---\n- a\n---\n")

	writeFile(t, filepath.Join(dir, "notes.md"), "---\n\nMeeting notes\n")

	broken := filepath.Join(dir, "broken.md")
	if err := os.Symlink("nowhere.md", broken); err != nil {
		t.Fatal(err)
	}

	old := warnOut
	warnOut = io.Discard

	t.Cleanup(func() { warnOut = old })

	_, skipped, err := LoadAllWithOptions(dir, ParseOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Not list.md or notes.md: they are readable and hold no item.
	want := []string{broken}
	if !slices.Equal(skipped, want) {
		t.Errorf("skipped = %q, want %q", skipped, want)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAll_SkipsBadFilesWithWarning(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string) string // returns the path that should be skipped
		want  string
	}{
		{
			name: "frontmatter is a scalar",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "README.md")
				writeFile(t, p, "---\nSome text\n---\n")

				return p
			},
			want: "frontmatter is not a YAML mapping",
		},
		{
			name: "frontmatter never closes",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "draft.md")
				writeFile(t, p, "---\nid: TK-001\n--- x\n")

				return p
			},
			want: "frontmatter has no closing --- line",
		},
		{
			name: "frontmatter is a sequence",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "list.md")
				writeFile(t, p, "---\n- a\n- b\n---\n")

				return p
			},
			want: "frontmatter is not a YAML mapping",
		},
		{
			name: "unreadable file",
			setup: func(t *testing.T, dir string) string {
				if os.Geteuid() == 0 {
					t.Skip("root can read any file")
				}

				p := filepath.Join(dir, "secret.md")
				writeFile(t, p, "---\nid: S-1\n---\n")

				if err := os.Chmod(p, 0); err != nil {
					t.Fatal(err)
				}

				t.Cleanup(func() { _ = os.Chmod(p, 0o600) })

				return p
			},
			want: "permission denied",
		},
		{
			name: "broken symlink",
			setup: func(t *testing.T, dir string) string {
				p := filepath.Join(dir, "broken.md")
				if err := os.Symlink("nowhere.md", p); err != nil {
					t.Fatal(err)
				}

				return p
			},
			want: "no such file or directory",
		},
		{
			name: "symlink to a directory named x.md",
			setup: func(t *testing.T, dir string) string {
				if err := os.Mkdir(filepath.Join(dir, "x.md"), 0o750); err != nil {
					t.Fatal(err)
				}

				p := filepath.Join(dir, "link.md")
				if err := os.Symlink("x.md", p); err != nil {
					t.Fatal(err)
				}

				return p
			},
			want: "is a directory",
		},
		{
			name: "unreadable subfolder",
			setup: func(t *testing.T, dir string) string {
				if os.Geteuid() == 0 {
					t.Skip("root can read any directory")
				}

				p := filepath.Join(dir, "locked")
				if err := os.Mkdir(p, 0o750); err != nil {
					t.Fatal(err)
				}

				writeFile(t, filepath.Join(p, "b.md"), "---\nid: B-1\n---\n")

				if err := os.Chmod(p, 0); err != nil {
					t.Fatal(err)
				}

				t.Cleanup(func() { _ = os.Chmod(p, 0o750) })

				return p
			},
			want: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "a.md"), "---\nid: A-1\ntitle: A\n---\n")
			skipped := tt.setup(t, dir)

			items, warnings, err := loadWithWarnings(t, dir)
			if err != nil {
				t.Fatalf("LoadAll: %v", err)
			}

			if len(items) != 1 || items[0].ID != "A-1" {
				t.Fatalf("expected only A-1 to load, got %v", items)
			}

			want := "warning: skipping " + skipped + ": " + tt.want + "\n"
			if warnings != want {
				t.Errorf("warnings = %q, want %q", warnings, want)
			}
		})
	}
}

func TestLoadAll_DirectoryNamedMdIsWalked(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "x.md")

	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(sub, "a.md"), "---\nid: A-1\n---\n")

	items, warnings, err := loadWithWarnings(t, dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(items) != 1 || warnings != "" {
		t.Errorf("got %d items and warnings %q, want 1 item and no warnings", len(items), warnings)
	}
}

// Invalid YAML may be an item with a typo, so it stays an error rather than a warning.
func TestLoadAll_InvalidYAMLStaysFatal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "---\nid: A-1\ntitle: [\n---\n")

	_, warnings, err := loadWithWarnings(t, dir)
	if err == nil {
		t.Fatal("expected an error")
	}

	if warnings != "" {
		t.Errorf("unexpected warnings %q", warnings)
	}
}
