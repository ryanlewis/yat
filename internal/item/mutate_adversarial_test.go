package item

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDraftFrontmatter = "---\nid: TK-001\nstatus: draft\n---\n"

// --- SetStatus adversarial tests ---

func TestSetStatus_EmptyStringStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(testDraftFrontmatter), 0o644)

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

func TestSetStatus_ReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "readonly.md")
	os.WriteFile(path, []byte(testDraftFrontmatter), 0o644)
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
status: draft
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
	content := "---\r\nid: TK-001\r\ntitle: \"Test\"\r\nstatus: draft\r\n---\r\n\r\nBody.\r\n"
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
	content := "---\nid: TK-001\nstatus: draft\nstatus: in-progress\n---\nBody.\n"
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	os.WriteFile(path, []byte(content), 0o644)

	if err := SetStatus(path, StatusDone); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	data, _ := os.ReadFile(path)
	result := string(data)
	// Both lines should be replaced
	if strings.Contains(result, "status: draft") || strings.Contains(result, "status: in-progress") {
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
	content := testDraftFrontmatter + body + "\n"
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
	content := "---\nid: TK-001\nstatus: draft\n---\n\n## 日本語テスト\n\n🚀 Emoji content 中文\n"
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
	os.WriteFile(path, []byte(testDraftFrontmatter), 0o644)

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
		{StatusDraft, StatusInProgress},
		{StatusInProgress, StatusDone},
		{StatusDone, StatusDraft},
		{StatusDraft, StatusDone},
		{StatusDone, StatusInProgress},
		{StatusInProgress, StatusDraft},
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
