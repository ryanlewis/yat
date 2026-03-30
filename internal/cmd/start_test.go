package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestStartCmd_ReadOnly(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)
	rc.ReadOnly = true

	cmd := &StartCmd{ID: "TK-003"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for read-only project")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected 'read-only' in error, got: %v", err)
	}
}

func TestStartCmd_Blocked(t *testing.T) {
	items := makeTestItems()
	// ST-001 depends on TK-001 (done) and TK-003 (todo) → blocked
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "ST-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for blocked item")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("expected 'blocked' in error, got: %v", err)
	}
}

func TestStartCmd_AlreadyDone(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "SP-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done' in error, got: %v", err)
	}
}

func TestStartCmd_AlreadyInProgress(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003, deps are done
	rc, buf := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "already in progress") {
		t.Errorf("expected 'already in progress' message, got:\n%s", buf.String())
	}
}

func TestStartCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestStartCmd_HappyPath_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: todo
dependencies: [TK-001]
---

Task body.
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusTodo, Dependencies: []string{"TK-001"}, FilePath: path, Body: "Task body.\n"},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &StartCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Started TK-002") {
		t.Errorf("expected 'Started TK-002', got:\n%s", out)
	}
	if !strings.Contains(out, "Task body.") {
		t.Errorf("expected body in output, got:\n%s", out)
	}

	// Verify file was updated on disc
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !strings.Contains(string(data), "status: in-progress") {
		t.Errorf("file not updated, got:\n%s", string(data))
	}
}

func TestStartCmd_HappyPath_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: todo
dependencies: [TK-001]
---

Task body.
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusTodo, Dependencies: []string{"TK-001"}, FilePath: path, Body: "Task body.\n"},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &StartCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result startJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "TK-002" {
		t.Errorf("ID = %q, want TK-002", result.ID)
	}
	if result.Title != "Abstraction" {
		t.Errorf("Title = %q, want Abstraction", result.Title)
	}
	if result.Body != "Task body.\n" {
		t.Errorf("Body = %q", result.Body)
	}
}

func TestStartCmd_CustomStatuses_AlreadyDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "shipped"},
	}
	rc, _ := newCustomTestContext(t, items, false)

	cmd := &StartCmd{ID: "A"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item with custom status")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done', got: %v", err)
	}
}

func TestStartCmd_CustomStatuses_AlreadyActive(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "review"},
	}
	rc, buf := newCustomTestContext(t, items, false)

	cmd := &StartCmd{ID: "A"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "already in progress") {
		t.Errorf("expected 'already in progress', got:\n%s", buf.String())
	}
}
