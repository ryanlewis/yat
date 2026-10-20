package item

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testTodoFrontmatter = "---\nid: TK-001\nstatus: todo\n---\n"

// --- SetStatus adversarial tests ---

func TestSetStatus_EmptyStringStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(testTodoFrontmatter), 0o644)

	err := SetStatus(path, Status(""))
	if err == nil {
		t.Fatal("expected error for empty status")
	}
}

func TestSetStatus_NonexistentFile(t *testing.T) {
	err := SetStatus("/nonexistent/path/file.md", StatusDone)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestSetStatus_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte("# Just a heading\nNo frontmatter here.\n"), 0o644)

	err := SetStatus(path, StatusDone)
	if err == nil {
		t.Fatal("expected error for file without frontmatter")
	}
	if !strings.Contains(err.Error(), "no frontmatter") {
		t.Errorf("expected 'no frontmatter' in error, got: %v", err)
	}
}

// skipIfRoot skips tests that rely on file modes, which root ignores.
func skipIfRoot(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("file modes do not restrict root")
	}
}

func TestSetStatus_ReadOnlyFile(t *testing.T) {
	skipIfRoot(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.md")
	os.WriteFile(path, []byte(testTodoFrontmatter), 0o644)
	os.Chmod(path, 0o444)
	t.Cleanup(func() { os.Chmod(path, 0o644) })

	err := SetStatus(path, StatusDone)
	if err == nil {
		t.Fatal("expected error for read-only file")
	}
}

func TestSetStatus_PreservesBody_WithStatusLikeContent(t *testing.T) {
	content := `---
id: TK-001
title: "Test"
status: todo
---

## Status update
status: in-progress
status: done
The word status: appears many times.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)

	// Frontmatter updated
	if !strings.Contains(result, "status: done") {
		t.Error("frontmatter not updated")
	}
	// Body preserved — all three body occurrences of status: remain
	count := strings.Count(result, "status: in-progress")
	if count != 1 {
		t.Errorf("body 'status: in-progress' count = %d, want 1", count)
	}
	if !strings.Contains(result, "The word status: appears many times.") {
		t.Error("body content was corrupted")
	}
}

func TestSetStatus_CRLFInput(t *testing.T) {
	content := "---\r\nid: TK-001\r\ntitle: \"Test\"\r\nstatus: todo\r\n---\r\n\r\nBody.\r\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusInProgress); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	if !strings.Contains(result, "status: in-progress") {
		t.Errorf("status not updated, got:\n%s", result)
	}
	// Body should still be present
	if !strings.Contains(result, "Body.") {
		t.Error("body was lost")
	}
}

func TestSetStatus_MultipleStatusFieldsInFrontmatter(t *testing.T) {
	// Pathological: two status fields in frontmatter (YAML gives last, regex replaces all)
	content := "---\nid: TK-001\nstatus: todo\nstatus: in-progress\n---\nBody.\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	// Both lines should be replaced
	if strings.Contains(result, "status: todo") || strings.Contains(result, "status: in-progress") {
		t.Errorf("not all status lines replaced, got:\n%s", result)
	}
}

func TestSetStatus_StatusWithExtraWhitespace(t *testing.T) {
	content := "---\nid: TK-001\nstatus:   draft\n---\nBody.\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("status not updated, got:\n%s", string(data))
	}
}

func TestSetStatus_Idempotent(t *testing.T) {
	content := "---\nid: TK-001\nstatus: done\n---\nBody.\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	// Setting to same status should still work
	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("status lost, got:\n%s", string(data))
	}
}

func TestSetStatus_LargeBody(t *testing.T) {
	body := strings.Repeat("This is a very long line of text. ", 1000)
	content := testTodoFrontmatter + body + "\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	if !strings.Contains(result, "status: done") {
		t.Error("status not updated")
	}
	if !strings.Contains(result, body) {
		t.Error("large body was corrupted")
	}
}

func TestSetStatus_UnicodeBody(t *testing.T) {
	content := "---\nid: TK-001\nstatus: todo\n---\n\n## 日本語テスト\n\n🚀 Emoji content 中文\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	if !strings.Contains(result, "🚀 Emoji content 中文") {
		t.Error("unicode body was corrupted")
	}
}

func TestSetStatus_EmptyBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(testTodoFrontmatter), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	if !strings.Contains(result, "status: done") {
		t.Error("status not updated")
	}
	// Should still have frontmatter delimiters
	if !strings.HasPrefix(result, "---") {
		t.Error("frontmatter delimiters lost")
	}
}

func TestSetStatus_AllTransitions(t *testing.T) {
	transitions := []struct {
		from Status
		to   Status
	}{
		{StatusTodo, StatusInProgress},
		{StatusInProgress, StatusDone},
		{StatusDone, StatusTodo},
		{StatusTodo, StatusDone},
		{StatusDone, StatusInProgress},
		{StatusInProgress, StatusTodo},
	}

	for _, tr := range transitions {
		t.Run(string(tr.from)+"_to_"+string(tr.to), func(t *testing.T) {
			content := "---\nid: TK-001\nstatus: " + string(tr.from) + "\n---\nBody.\n"
			dir := t.TempDir()
			path := filepath.Join(dir, "test.md")
			os.WriteFile(path, []byte(content), 0o644)

			if err := SetStatus(path, tr.to); err != nil {
				t.Fatalf("SetStatus(%s -> %s): %v", tr.from, tr.to, err)
			}

			data, _ := os.ReadFile(path)
			if !strings.Contains(string(data), "status: "+string(tr.to)) {
				t.Errorf("status not updated to %s, got:\n%s", tr.to, string(data))
			}
		})
	}
}

// --- Byte preservation regressions: SetStatus must change only the status value ---

func TestSetStatus_PreservesSurroundingBytes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "CRLF closing delimiter",
			input: "---\r\nid: TK-001\r\nstatus: todo\r\n---\r\nBody.\r\n",
			want:  "---\r\nid: TK-001\r\nstatus: in-progress\r\n---\r\nBody.\r\n",
		},
		{
			name:  "no trailing newline after closing delimiter",
			input: "---\nid: TK-001\nstatus: todo\n---",
			want:  "---\nid: TK-001\nstatus: in-progress\n---",
		},
		{
			name:  "UTF-8 BOM",
			input: "\xef\xbb\xbf---\nid: TK-001\nstatus: todo\n---\nBody.\n",
			want:  "\xef\xbb\xbf---\nid: TK-001\nstatus: in-progress\n---\nBody.\n",
		},
		{
			name:  "leading blank lines",
			input: "\n\n---\nid: TK-001\nstatus: todo\n---\nBody.\n",
			want:  "\n\n---\nid: TK-001\nstatus: in-progress\n---\nBody.\n",
		},
		{
			name:  "empty status value does not swallow the next line",
			input: "---\nid: TK-001\nstatus:\ntitle: Alpha\n---\nBody.\n",
			want:  "---\nid: TK-001\nstatus: in-progress\ntitle: Alpha\n---\nBody.\n",
		},
		{
			name:  "empty status value with CRLF",
			input: "---\r\nid: TK-001\r\nstatus:\r\ntitle: Alpha\r\n---\r\n",
			want:  "---\r\nid: TK-001\r\nstatus: in-progress\r\ntitle: Alpha\r\n---\r\n",
		},
		{
			name:  "empty status value with comment",
			input: "---\nid: TK-001\nstatus: # pick later\n---\n",
			want:  "---\nid: TK-001\nstatus: in-progress # pick later\n---\n",
		},
		{
			name:  "status value with trailing comment",
			input: "---\nid: TK-001\nstatus: todo # was draft\n---\n",
			want:  "---\nid: TK-001\nstatus: in-progress # was draft\n---\n",
		},
		{
			name:  "comment kept when value is on the next line",
			input: "---\nid: TK-001\nstatus: # pick later\n  todo\n---\n",
			want:  "---\nid: TK-001\nstatus: in-progress # pick later\n---\n",
		},
		{
			name:  "status value on the next line",
			input: "---\nid: TK-001\nstatus:\n  todo\ntitle: Alpha\n---\n",
			want:  "---\nid: TK-001\nstatus: in-progress\ntitle: Alpha\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.md")
			if err := os.WriteFile(path, []byte(tt.input), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := SetStatus(path, StatusInProgress); err != nil {
				t.Fatalf("SetStatus: %v", err)
			}

			data, _ := os.ReadFile(path)
			if string(data) != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", data, tt.want)
			}
		})
	}
}

func TestSetStatusWithOptions_MultiWordStatus(t *testing.T) {
	opts := MutateOptions{ValidStatuses: NewStatusSet([]string{"not started", "in flight", "shipped"})}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "double-quoted",
			input: "---\nid: TK-001\nstatus: \"not started\"\n---\n",
			want:  "---\nid: TK-001\nstatus: in flight\n---\n",
		},
		{
			name:  "single-quoted",
			input: "---\nid: TK-001\nstatus: 'not started'\n---\n",
			want:  "---\nid: TK-001\nstatus: in flight\n---\n",
		},
		{
			name:  "plain",
			input: "---\nid: TK-001\nstatus: not started # note\n---\n",
			want:  "---\nid: TK-001\nstatus: in flight # note\n---\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.md")
			if err := os.WriteFile(path, []byte(tt.input), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := SetStatusWithOptions(path, Status("in flight"), opts); err != nil {
				t.Fatalf("SetStatusWithOptions: %v", err)
			}

			data, _ := os.ReadFile(path)
			if string(data) != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", data, tt.want)
			}

			it, err := ParseWithOptions(data, path, ParseOptions{ValidStatuses: opts.ValidStatuses})
			if err != nil || it == nil || it.Status != "in flight" {
				t.Errorf("rewritten file does not parse back to \"in flight\": item=%+v err=%v", it, err)
			}
		})
	}
}

func TestSetStatus_RefusesRewriteThatWouldNotParseBack(t *testing.T) {
	// Status values the line rewrite cannot replace cleanly must be left alone
	// with an error, never written as a corrupted file.
	tests := []struct {
		name  string
		input string
	}{
		{name: "multi-line plain", input: "---\nid: TK-001\nstatus: to\n  do\n---\n"},
		{name: "folded block scalar", input: "---\nid: TK-001\nstatus: >\n  todo\n---\n"},
		{name: "literal block scalar", input: "---\nid: TK-001\nstatus: |-\n  todo\n---\n"},
		{name: "two continuation lines", input: "---\nid: TK-001\nstatus:\n  to\n  do\n---\n"},
		{name: "anchored value", input: "---\nid: TK-001\nstatus: &s todo\nphase: *s\n---\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.md")
			if err := os.WriteFile(path, []byte(tt.input), 0o644); err != nil {
				t.Fatal(err)
			}

			if err := SetStatus(path, StatusInProgress); err == nil {
				data, _ := os.ReadFile(path)
				t.Fatalf("expected error, file now:\n%q", data)
			}

			data, _ := os.ReadFile(path)
			if string(data) != tt.input {
				t.Errorf("file changed:\n%q", data)
			}
		})
	}
}

// --- atomic write tests ---

// assertNoTempFiles fails if anything other than want is left in dir.
func assertNoTempFiles(t *testing.T, dir string, want ...string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("dir entries = %v, want %v", got, want)
	}
}

func TestSetStatus_KeepsFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(testTodoFrontmatter), 0o644)
	os.Chmod(path, 0o640)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v, want 0640", info.Mode().Perm())
	}
	assertNoTempFiles(t, dir, "test.md")
}

func TestSetStatus_WritesThroughSymlink(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "real")
	os.Mkdir(targetDir, 0o755)
	target := filepath.Join(targetDir, "test.md")
	os.WriteFile(target, []byte(testTodoFrontmatter), 0o644)
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(filepath.Join("real", "test.md"), link); err != nil {
		t.Fatal(err)
	}

	if err := SetStatus(link, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link.md is no longer a symlink: mode %v", info.Mode())
	}

	data, _ := os.ReadFile(target)
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("target not updated, got:\n%s", data)
	}
	assertNoTempFiles(t, dir, "link.md", "real")
	assertNoTempFiles(t, targetDir, "test.md")
}

func TestSetStatus_ReadOnlyDirWritesInPlace(t *testing.T) {
	skipIfRoot(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(testTodoFrontmatter), 0o644)
	// A read-only directory stops the temp file being created, so the item is
	// written in place instead.
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("status not updated, got:\n%s", data)
	}
	assertNoTempFiles(t, dir, "test.md")
}
