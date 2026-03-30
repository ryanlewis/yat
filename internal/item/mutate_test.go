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
