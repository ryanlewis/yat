package item

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Malformed YAML ---

func TestParse_MalformedYAML(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantErr bool
	}{
		{
			name:    "unclosed YAML string",
			input:   "---\nid: TK-001\ntitle: \"unclosed\n---\n",
			wantErr: true,
		},
		{
			name:    "tab indentation in YAML",
			input:   "---\nid: TK-001\n\ttitle: \"Tabs\"\nstatus: draft\n---\n",
			wantErr: true,
		},
		{
			name:    "duplicate keys in YAML rejected",
			input:   "---\nid: TK-001\nid: TK-002\nstatus: draft\n---\n",
			wantErr: true, // yaml.v3 is strict about duplicate keys
		},
		{
			name:    "YAML anchor and alias",
			input:   "---\nid: TK-001\ntitle: &t \"Anchor\"\nalias: *t\nstatus: draft\n---\n",
			wantErr: false,
		},
		{
			name:    "YAML with colon in unquoted value",
			input:   "---\nid: TK-001\ntitle: This: has a colon\nstatus: draft\n---\n",
			wantErr: true, // YAML rejects unquoted colons in values
		},
		{
			name:    "YAML with multiline string",
			input:   "---\nid: TK-001\ntitle: |\n  Line 1\n  Line 2\nstatus: draft\n---\n",
			wantErr: false,
		},
		{
			name:    "YAML with flow mapping (ignored fields)",
			input:   "---\nid: TK-001\nextra: {nested: value}\nstatus: draft\n---\n",
			wantErr: false,
		},
		{
			name:    "YAML null id with status triggers ErrMissingID",
			input:   "---\nid: null\nstatus: draft\n---\n",
			wantErr: true, // id="" + status="draft" → looksLikeItem → ErrMissingID
		},
		{
			name:    "YAML empty id string with status triggers ErrMissingID",
			input:   "---\nid: \"\"\nstatus: draft\n---\n",
			wantErr: true, // id="" + status="draft" → looksLikeItem → ErrMissingID
		},
		{
			name:    "YAML boolean id",
			input:   "---\nid: true\nstatus: draft\n---\n",
			wantErr: false, // YAML unmarshals true to string "true"
		},
		{
			name:    "YAML numeric id",
			input:   "---\nid: 12345\nstatus: draft\n---\n",
			wantErr: false, // YAML unmarshals 12345 to string "12345"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (item=%+v)", item)
				}
				return
			}
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil item with no error, got err: %v", err)
				}
				if item != nil {
					t.Fatalf("expected nil, got %+v", item)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
		})
	}
}

// --- Frontmatter delimiter edge cases ---

func TestParse_DelimiterEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantErr bool
		wantID  string
	}{
		{
			name:    "empty frontmatter block",
			input:   "---\n---\n",
			wantNil: true,
		},
		{
			name:    "whitespace-only frontmatter",
			input:   "---\n   \n---\n",
			wantNil: true,
		},
		{
			name:   "multiple --- in body preserved",
			input:  "---\nid: TK-001\nstatus: draft\n---\n\n---\nThis is a separator\n---\n",
			wantID: "TK-001",
		},
		{
			name:   "leading newlines before frontmatter",
			input:  "\n\n\n---\nid: TK-001\nstatus: draft\n---\n",
			wantID: "TK-001",
		},
		{
			name:   "leading CRLF before frontmatter",
			input:  "\r\n\r\n---\nid: TK-001\nstatus: draft\n---\n",
			wantID: "TK-001",
		},
		{
			name:    "four dashes is not a delimiter",
			input:   "----\nid: TK-001\n---\n",
			wantNil: true,
		},
		{
			name:    "two dashes is not a delimiter",
			input:   "--\nid: TK-001\n---\n",
			wantNil: true,
		},
		{
			name:    "closing delimiter with trailing spaces",
			input:   "---\nid: TK-001\nstatus: draft\n--- \n",
			wantNil: true, // "--- " is not "---" followed by \n/\r/EOF
		},
		{
			name:   "closing delimiter at exact EOF",
			input:  "---\nid: TK-001\nstatus: draft\n---",
			wantID: "TK-001",
		},
		{
			name:    "CR-only line endings",
			input:   "---\rid: TK-001\rstatus: draft\r---\r",
			wantNil: true, // opening --- must be followed by \n or \r, and \r is valid...
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil with no error, got err: %v", err)
				}
				if item != nil {
					t.Fatalf("expected nil, got %+v", item)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
			if item.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", item.ID, tt.wantID)
			}
		})
	}
}

// --- Unicode and special characters ---

func TestParse_Unicode(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantID    string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "unicode in title",
			input:     "---\nid: TK-001\ntitle: \"Ünïcödé tïtlé\"\nstatus: draft\n---\n\nBody.\n",
			wantID:    "TK-001",
			wantTitle: "Ünïcödé tïtlé",
		},
		{
			name:   "emoji in ID",
			input:  "---\nid: \"\\U0001F680-launch\"\nstatus: draft\n---\n",
			wantID: "🚀-launch",
		},
		{
			name:     "CJK characters in body",
			input:    "---\nid: TK-001\nstatus: draft\n---\n\n日本語テスト\n",
			wantID:   "TK-001",
			wantBody: "\n日本語テスト\n",
		},
		{
			name:      "RTL text in title",
			input:     "---\nid: TK-001\ntitle: \"مرحبا\"\nstatus: draft\n---\n",
			wantID:    "TK-001",
			wantTitle: "مرحبا",
		},
		{
			name:   "ID with spaces (quoted)",
			input:  "---\nid: \"has spaces\"\nstatus: draft\n---\n",
			wantID: "has spaces",
		},
		{
			name:   "ID with special YAML chars",
			input:  "---\nid: \"colon:slash/bracket[]\"\nstatus: draft\n---\n",
			wantID: "colon:slash/bracket[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
			if item.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", item.ID, tt.wantID)
			}
			if tt.wantTitle != "" && item.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", item.Title, tt.wantTitle)
			}
			if tt.wantBody != "" && item.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", item.Body, tt.wantBody)
			}
		})
	}
}

// --- Status and Priority edge cases ---

func TestParse_StatusPriorityEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantNil bool
	}{
		{
			name:    "status with leading whitespace in value",
			input:   "---\nid: TK-001\nstatus: \" draft\"\n---\n",
			wantErr: true, // " draft" is not valid
		},
		{
			name:    "priority lowercase critical",
			input:   "---\nid: TK-001\npriority: critical\n---\n",
			wantErr: true, // case-sensitive
		},
		{
			name:    "priority CRITICAL uppercase",
			input:   "---\nid: TK-001\npriority: CRITICAL\n---\n",
			wantErr: true,
		},
		{
			name:    "priority Mixed case",
			input:   "---\nid: TK-001\npriority: hIGH\n---\n",
			wantErr: true,
		},
		{
			name:    "status Draft capitalized",
			input:   "---\nid: TK-001\nstatus: Draft\n---\n",
			wantErr: true,
		},
		{
			name:    "status DONE uppercase",
			input:   "---\nid: TK-001\nstatus: DONE\n---\n",
			wantErr: true,
		},
		{
			name:    "status in_progress with underscore",
			input:   "---\nid: TK-001\nstatus: in_progress\n---\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (item=%+v)", item)
				}
				return
			}
			if tt.wantNil {
				if err != nil {
					t.Fatalf("expected nil with no error, got err: %v", err)
				}
				if item != nil {
					t.Fatalf("expected nil, got %+v", item)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// --- Points boundary conditions ---

func TestParse_PointsBoundary(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPoints int
		wantErr    bool
	}{
		{
			name:       "zero points",
			input:      "---\nid: TK-001\npoints: 0\nstatus: draft\n---\n",
			wantPoints: 0,
		},
		{
			name:       "negative points",
			input:      "---\nid: TK-001\npoints: -5\nstatus: draft\n---\n",
			wantPoints: -5,
		},
		{
			name:       "large points",
			input:      "---\nid: TK-001\npoints: 999999\nstatus: draft\n---\n",
			wantPoints: 999999,
		},
		{
			name:       "float points silently truncated",
			input:      "---\nid: TK-001\npoints: 3.5\nstatus: draft\n---\n",
			wantPoints: 3, // YAML silently truncates float to int
		},
		{
			name:    "string points",
			input:   "---\nid: TK-001\npoints: \"three\"\nstatus: draft\n---\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (item=%+v)", item)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
			if item.Points != tt.wantPoints {
				t.Errorf("Points = %d, want %d", item.Points, tt.wantPoints)
			}
		})
	}
}

// --- Dependencies edge cases ---

func TestParse_DependenciesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDeps []string
	}{
		{
			name:     "nil dependencies (not specified)",
			input:    "---\nid: TK-001\nstatus: draft\n---\n",
			wantDeps: nil,
		},
		{
			name:     "empty list",
			input:    "---\nid: TK-001\ndependencies: []\nstatus: draft\n---\n",
			wantDeps: []string{},
		},
		{
			name:     "single dependency",
			input:    "---\nid: TK-001\ndependencies: [A]\nstatus: draft\n---\n",
			wantDeps: []string{"A"},
		},
		{
			name:     "multiline list syntax",
			input:    "---\nid: TK-001\ndependencies:\n  - A\n  - B\n  - C\nstatus: draft\n---\n",
			wantDeps: []string{"A", "B", "C"},
		},
		{
			name:     "duplicate dependencies",
			input:    "---\nid: TK-001\ndependencies: [A, A, A]\nstatus: draft\n---\n",
			wantDeps: []string{"A", "A", "A"}, // no dedup at parse level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
			if tt.wantDeps == nil {
				if item.Dependencies != nil {
					t.Errorf("Dependencies = %v, want nil", item.Dependencies)
				}
			} else {
				if len(item.Dependencies) != len(tt.wantDeps) {
					t.Fatalf("Dependencies = %v, want %v", item.Dependencies, tt.wantDeps)
				}
				for i, dep := range tt.wantDeps {
					if item.Dependencies[i] != dep {
						t.Errorf("Dependencies[%d] = %q, want %q", i, item.Dependencies[i], dep)
					}
				}
			}
		})
	}
}

// --- Body preservation edge cases ---

func TestParse_BodyPreservation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBody string
	}{
		{
			name:     "empty body",
			input:    "---\nid: TK-001\nstatus: draft\n---\n",
			wantBody: "",
		},
		{
			name:     "body with frontmatter-like markers",
			input:    "---\nid: TK-001\nstatus: draft\n---\n---\nfake frontmatter\n---\n",
			wantBody: "---\nfake frontmatter\n---\n",
		},
		{
			name:     "body with YAML content",
			input:    "---\nid: TK-001\nstatus: draft\n---\nid: fake\nstatus: in-progress\n",
			wantBody: "id: fake\nstatus: in-progress\n",
		},
		{
			name:     "body with only newlines",
			input:    "---\nid: TK-001\nstatus: draft\n---\n\n\n\n",
			wantBody: "\n\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item == nil {
				t.Fatal("expected non-nil item")
			}
			if item.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", item.Body, tt.wantBody)
			}
		})
	}
}

// --- looksLikeItem edge cases ---

func TestParse_LooksLikeItem(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "has type but no id",
			input:   "---\ntype: Task\n---\n",
			wantErr: true, // ErrMissingID
		},
		{
			name:    "has status but no id",
			input:   "---\nstatus: draft\n---\n",
			wantErr: true,
		},
		{
			name:    "has priority but no id",
			input:   "---\npriority: High\n---\n",
			wantErr: true,
		},
		{
			name:    "has dependencies but no id",
			input:   "---\ndependencies: [A]\n---\n",
			wantErr: true,
		},
		{
			name:    "has only title (not item-like, no id) returns nil",
			input:   "---\ntitle: \"Just a doc\"\n---\n",
			wantErr: false, // returns nil, nil
		},
		{
			name:    "has only points (not item-like) returns nil",
			input:   "---\npoints: 5\n---\n",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, ErrMissingID) {
					t.Errorf("expected ErrMissingID, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if item != nil {
				t.Fatalf("expected nil, got %+v", item)
			}
		})
	}
}

// --- LoadAll edge cases ---

func TestLoadAll_NonexistentDir(t *testing.T) {
	_, err := LoadAll("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestLoadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestLoadAll_NonMDFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not markdown"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "item.md"), []byte("---\nid: TK-001\nstatus: draft\n---\n"), 0o644)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestLoadAll_NestedDirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "phase1", "tasks")
	os.MkdirAll(sub, 0o755)

	os.WriteFile(filepath.Join(dir, "root.md"), []byte("---\nid: ROOT\nstatus: draft\n---\n"), 0o644)
	os.WriteFile(filepath.Join(sub, "nested.md"), []byte("---\nid: NESTED\nstatus: draft\n---\n"), 0o644)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestLoadAll_InvalidItemStopsLoad(t *testing.T) {
	dir := t.TempDir()
	// Valid item
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nid: A\nstatus: draft\n---\n"), 0o644)
	// Invalid item (bad status)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\nid: B\nstatus: banana\n---\n"), 0o644)

	_, err := LoadAll(dir)
	if err == nil {
		t.Fatal("expected error from invalid item")
	}
	if !strings.Contains(err.Error(), "banana") {
		t.Errorf("expected error about 'banana', got: %v", err)
	}
}

func TestLoadAll_MissingIDWarnsButContinues(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("---\nid: A\nstatus: draft\n---\n"), 0o644)
	// Item-like but no ID — should warn but not fail
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("---\ntype: Task\nstatus: draft\n---\n"), 0o644)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (skipping missing ID), got %d", len(items))
	}
}

func TestLoadAll_MDWithNoFrontmatterSkipped(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# README\nJust a doc.\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "item.md"), []byte("---\nid: TK-001\nstatus: draft\n---\n"), 0o644)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

// --- Index edge cases ---

func TestIndex_EmptySlice(t *testing.T) {
	idx, err := Index([]*Item{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}
}

func TestIndex_NilSlice(t *testing.T) {
	idx, err := Index(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}
}

func TestIndex_DuplicateIDErrorIncludesPaths(t *testing.T) {
	items := []*Item{
		{ID: "TK-001", FilePath: "path/a.md"},
		{ID: "TK-001", FilePath: "path/b.md"},
	}
	_, err := Index(items)
	if err == nil {
		t.Fatal("expected error for duplicate IDs")
	}
	if !strings.Contains(err.Error(), "path/a.md") || !strings.Contains(err.Error(), "path/b.md") {
		t.Errorf("error should include both file paths, got: %v", err)
	}
}

// --- ParseFile edge cases ---

func TestParseFile_NonexistentFile(t *testing.T) {
	_, err := ParseFile("/nonexistent/file.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	os.WriteFile(path, []byte(""), 0o644)

	item, err := ParseFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Fatalf("expected nil for empty file, got %+v", item)
	}
}

func TestParseFile_BinaryContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.md")
	os.WriteFile(path, []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}, 0o644)

	item, err := ParseFile(path)
	if err != nil {
		// Binary content shouldn't crash — either returns nil or an error
		return
	}
	if item != nil {
		t.Fatalf("expected nil for binary file, got %+v", item)
	}
}

// --- yat directives ---

func TestParse_YatIgnore(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{
			name:    "yat ignore skips file",
			input:   "---\nid: TK-001\nstatus: draft\nyat: ignore\n---\nBody.\n",
			wantNil: true,
		},
		{
			name:    "yat ignore with spaces",
			input:   "---\nid: TK-001\nstatus: draft\nyat:   ignore  \n---\n",
			wantNil: true,
		},
		{
			name:    "yat ignore in comma list",
			input:   "---\nid: TK-001\nstatus: draft\nyat: \"something, ignore, other\"\n---\n",
			wantNil: true,
		},
		{
			name:    "yat with other directive does not skip",
			input:   "---\nid: TK-001\nstatus: draft\nyat: draft\n---\n",
			wantNil: false,
		},
		{
			name:    "no yat field does not skip",
			input:   "---\nid: TK-001\nstatus: draft\n---\n",
			wantNil: false,
		},
		{
			name:    "yat ignore on non-item file",
			input:   "---\ntitle: Notes\nyat: ignore\n---\nJust notes.\n",
			wantNil: true,
		},
		{
			name:    "yat ignore prevents ErrMissingID",
			input:   "---\nstatus: draft\ntype: Task\nyat: ignore\n---\n",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if item != nil {
					t.Fatalf("expected nil (ignored), got %+v", item)
				}
			} else {
				if item == nil {
					t.Fatal("expected non-nil item")
				}
			}
		})
	}
}

func TestLoadAll_YatIgnoreSkipsFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "item.md"), []byte("---\nid: TK-001\nstatus: draft\n---\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("---\nid: NOTES-001\nstatus: draft\nyat: ignore\n---\nMeeting notes.\n"), 0o644)

	items, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (notes ignored), got %d", len(items))
	}
	if items[0].ID != "TK-001" {
		t.Errorf("expected TK-001, got %s", items[0].ID)
	}
}

// --- Type validation (or lack thereof) ---

func TestParse_ArbitraryTypeAccepted(t *testing.T) {
	input := "---\nid: TK-001\ntype: CompletelyMadeUpType\nstatus: draft\n---\n"
	item, err := Parse([]byte(input), "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Type != "CompletelyMadeUpType" {
		t.Errorf("Type = %q, want CompletelyMadeUpType", item.Type)
	}
}

// --- Phase validation (or lack thereof) ---

func TestParse_ArbitraryPhaseAccepted(t *testing.T) {
	input := "---\nid: TK-001\nphase: \"not-a-number\"\nstatus: draft\n---\n"
	item, err := Parse([]byte(input), "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Phase != "not-a-number" {
		t.Errorf("Phase = %q, want not-a-number", item.Phase)
	}
}
