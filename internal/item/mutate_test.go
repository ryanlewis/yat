package item

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetStatus(t *testing.T) {
	// Create a temp file with frontmatter
	content := `---
id: TK-001
title: "Test task"
type: Task
priority: Critical
points: 3
dependencies: []
status: todo
phase: "1"
---

## Body content
This should be preserved exactly.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	// Change status to in-progress
	if err := SetStatus(path, StatusInProgress); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	// Read back and verify
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	result := string(data)

	if !strings.Contains(result, "status: in-progress") {
		t.Errorf("status not updated, got:\n%s", result)
	}

	// Body should be preserved
	if !strings.Contains(result, "## Body content\nThis should be preserved exactly.") {
		t.Errorf("body not preserved, got:\n%s", result)
	}

	// Other fields should be preserved
	if !strings.Contains(result, "id: TK-001") {
		t.Errorf("id field lost, got:\n%s", result)
	}

	// Now change to done
	err = SetStatus(path, StatusDone)
	if err != nil {
		t.Fatalf("SetStatus to done: %v", err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if !strings.Contains(string(data), "status: done") {
		t.Errorf("status not updated to done, got:\n%s", string(data))
	}
}

func TestSetStatusInvalid(t *testing.T) {
	content := `---
id: TK-001
title: "Test"
status: todo
---

Body.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	err := SetStatus(path, Status("banana"))
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestSetStatusNoField(t *testing.T) {
	content := `---
id: TK-001
title: "No status field"
---

Body.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	err := SetStatus(path, StatusDone)
	if err == nil {
		t.Fatal("expected error for missing status field")
	}
}

func TestSetStatusWithOptions_AliasedField(t *testing.T) {
	content := `---
id: TK-001
title: "Test"
state: backlog
---

Body.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	opts := MutateOptions{
		FieldName:     "state",
		ValidStatuses: NewStatusSet([]string{"backlog", "review", "shipped"}),
	}

	if err := SetStatusWithOptions(path, "review", opts); err != nil {
		t.Fatalf("SetStatusWithOptions: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	result := string(data)
	if !strings.Contains(result, "state: review") {
		t.Errorf("state not updated, got:\n%s", result)
	}
	if strings.Contains(result, "state: backlog") {
		t.Errorf("old state still present, got:\n%s", result)
	}
}

func TestSetStatusWithOptions_CustomValidation(t *testing.T) {
	content := `---
id: TK-001
status: todo
---
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	opts := MutateOptions{
		ValidStatuses: NewStatusSet([]string{"draft", "review", "shipped"}),
	}

	err := SetStatusWithOptions(path, "banana", opts)
	if err == nil {
		t.Fatal("expected error for invalid custom status")
	}
}

func TestSetStatusInBody(t *testing.T) {
	// Verify that a "status:" line in the body is NOT rewritten
	content := `---
id: TK-001
title: "Test"
status: todo
---

## Example YAML
status: pending
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	result := string(data)

	// Frontmatter should be updated
	if !strings.Contains(result, "status: done") {
		t.Errorf("frontmatter status not updated, got:\n%s", result)
	}

	// Body should still contain the original "status: pending" line
	if !strings.Contains(result, "status: pending") {
		t.Errorf("body status line was incorrectly rewritten, got:\n%s", result)
	}
}

func TestSetStatusWithOptions_Alias(t *testing.T) {
	tests := []struct {
		name    string
		fm      string
		want    string
		wantErr string
	}{
		{name: "canonical only", fm: "id: A\nstatus: todo\n", want: "id: A\nstatus: done\n"},
		{name: "alias only", fm: "id: A\nstate: todo\n", want: "id: A\nstate: done\n"},
		// parse reads status first when both are present, so that is the key written.
		{name: "both", fm: "id: A\nstate: todo\nstatus: todo\n", want: "id: A\nstate: todo\nstatus: done\n"},
		{name: "neither", fm: "id: A\n", wantErr: "no status or state field found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.md")
			if err := os.WriteFile(path, []byte("---\n"+tt.fm+"---\n\nBody.\n"), 0o644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}

			err := SetStatusWithOptions(path, StatusDone, MutateOptions{FieldName: "status", Alias: "state"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("SetStatusWithOptions: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading back: %v", err)
			}

			if want := "---\n" + tt.want + "---\n\nBody.\n"; string(data) != want {
				t.Errorf("got:\n%s\nwant:\n%s", data, want)
			}
		})
	}
}

// A quoted or spaced key loads, so it must also be written, keeping its spelling.
func TestSetStatusWithOptions_QuotedOrSpacedKey(t *testing.T) {
	tests := []struct {
		name  string
		field string
		fm    string
		want  string
	}{
		{name: "double-quoted", field: "status", fm: "id: A\n\"status\": todo\n", want: "id: A\n\"status\": done\n"},
		{name: "single-quoted", field: "status", fm: "id: A\n'status': todo # c\n", want: "id: A\n'status': done # c\n"},
		{name: "spaced", field: "status", fm: "id: A\nstatus : todo\n", want: "id: A\nstatus : done\n"},
		{name: "quoted and spaced", field: "status", fm: "id: A\n\"status\"  :\ttodo\n", want: "id: A\n\"status\"  : done\n"},
		{name: "quoted alias", field: "state", fm: "id: A\n\"state\": todo\n", want: "id: A\n\"state\": done\n"},
		{name: "value on next line", field: "status", fm: "id: A\n\"status\":\n  todo\n", want: "id: A\n\"status\": done\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.md")
			if err := os.WriteFile(path, []byte("---\n"+tt.fm+"---\n\nBody.\n"), 0o644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}

			if err := SetStatusWithOptions(path, StatusDone, MutateOptions{FieldName: tt.field}); err != nil {
				t.Fatalf("SetStatusWithOptions: %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading back: %v", err)
			}

			if want := "---\n" + tt.want + "---\n\nBody.\n"; string(data) != want {
				t.Errorf("got:\n%q\nwant:\n%q", data, want)
			}
		})
	}
}

// A key written with escapes or in explicit "? key" form loads, so it must also
// be written, keeping its spelling; a value that cannot be found byte for byte
// is refused, leaving the file untouched.
func TestSetStatusWithOptions_EscapedOrExplicitKey(t *testing.T) {
	tests := []struct {
		name  string
		field string
		fm    string
		want  string // "" when the write is refused
	}{
		{name: "escaped", field: "status", fm: "id: A\n\"st\\x61tus\": todo # c\n", want: "id: A\n\"st\\x61tus\": done # c\n"},
		{name: "escaped alias", field: "state", fm: "id: A\n\"st\\x61te\": todo\n", want: "id: A\n\"st\\x61te\": done\n"},
		{name: "explicit", field: "status", fm: "id: A\n? status\n: todo\n", want: "id: A\n? status\n: done\n"},
		{name: "explicit quoted value", field: "status", fm: "? status\n: 'todo' # c\nid: A\n", want: "? status\n: done # c\nid: A\n"},
		{name: "explicit value on next line", field: "status", fm: "? status\n:\n  todo\n", want: "? status\n:\n  done\n"},
		{name: "explicit CRLF", field: "status", fm: "id: A\r\n? status\r\n: todo\r\n", want: "id: A\r\n? status\r\n: done\r\n"},
		{
			name:  "after a line separator",
			field: "status",
			fm:    "title: \"a\u2028b\"\n? status\n: todo\n",
			want:  "title: \"a\u2028b\"\n? status\n: done\n",
		},
		{name: "anchored value", field: "status", fm: "? status\n: &s todo\n"},
		{name: "tagged value", field: "status", fm: "? status\n: !!str todo\n"},
		{name: "multi-line value", field: "status", fm: "? status\n: in\n  progress\n"},
		{name: "multi-line quoted value", field: "status", fm: "? status\n: \"to\n  do\"\n"},
		{name: "escaped empty value", field: "status", fm: "id: A\n\"st\\x61tus\":\n", want: "id: A\n\"st\\x61tus\": done\n"},
		{
			name: "escaped empty value with comment", field: "status", fm: "\"st\\x61tus\":   # c\nid: A\n",
			want: "\"st\\x61tus\": done # c\nid: A\n",
		},
		{name: "escaped quoted empty value", field: "status", fm: "\"st\\x61tus\": \"\"\n", want: "\"st\\x61tus\": done\n"},
		{name: "explicit empty value", field: "status", fm: "? status\n:\nid: A\n", want: "? status\n: done\nid: A\n"},
		{name: "explicit empty CRLF", field: "status", fm: "? status\r\n:  \r\nid: A\r\n", want: "? status\r\n: done\r\nid: A\r\n"},
		{name: "empty value", field: "status", fm: "id: A\n? status\n"},
		{name: "empty value in flow mapping", field: "status", fm: "{id: A, \"st\\x61tus\": }\n"},
		{name: "flow mapping", field: "status", fm: "{id: A, status: todo}\n"},
		{name: "alias node key", field: "status", fm: "id: A\nx: &s status\n*s : todo\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a.md")
			orig := "---\n" + tt.fm + "---\n\nBody.\n"
			if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
				t.Fatalf("writing test file: %v", err)
			}

			err := SetStatusWithOptions(path, StatusDone, MutateOptions{FieldName: tt.field})

			want := orig
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), "status key form not supported for writing") {
					t.Errorf("err = %v, want a key form error", err)
				}
			} else {
				if err != nil {
					t.Fatalf("SetStatusWithOptions: %v", err)
				}

				want = "---\n" + tt.want + "---\n\nBody.\n"
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading back: %v", err)
			}

			if string(data) != want {
				t.Errorf("got:\n%q\nwant:\n%q", data, want)
			}
		})
	}
}

// An alias used as a key (*status) is not the status key, though its node holds
// the name "status"; writing it would leave the status unchanged.
func TestSetStatus_AliasNodeKeyIsNotTheStatusKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.md")
	orig := "---\nid: A\nx: &status foo\n*status : todo\n---\n\nBody.\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	if err := SetStatus(path, StatusDone); err == nil {
		t.Error("SetStatus succeeded, want an error")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}

	if string(data) != orig {
		t.Errorf("file changed:\n%q", data)
	}
}
