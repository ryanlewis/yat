package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestCompleteCmd_ReadOnly(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)
	rc.ReadOnly = true

	cmd := &CompleteCmd{ID: "TK-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for read-only project")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("expected 'read-only' in error, got: %v", err)
	}
}

func TestCompleteCmd_AlreadyDone(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "SP-001"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item")
	}
	if !strings.Contains(err.Error(), "already done") {
		t.Errorf("expected 'already done' in error, got: %v", err)
	}
}

func TestCompleteCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestCompleteCmd_HappyPath_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-003.md", `---
id: TK-003
title: "Error handling"
type: Task
priority: High
status: todo
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-003", Title: "Error handling", Status: item.StatusTodo, Priority: item.PriorityHigh, Dependencies: []string{"TK-001"}, FilePath: path},
		{ID: "ST-001", Title: "Command parsing", Status: item.StatusTodo, Dependencies: []string{"TK-001", "TK-003"}},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed TK-003") {
		t.Errorf("expected 'Completed TK-003', got:\n%s", out)
	}
	// ST-001 depends on TK-001 (done) and TK-003 (now done) → should be unblocked
	if !strings.Contains(out, "Now unblocked") {
		t.Errorf("expected 'Now unblocked' section, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-001") {
		t.Errorf("expected ST-001 in unblocked list, got:\n%s", out)
	}

	// Verify file was updated on disc
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !strings.Contains(string(data), "status: done") {
		t.Errorf("file not updated, got:\n%s", string(data))
	}
}

func TestCompleteCmd_HappyPath_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-003.md", `---
id: TK-003
title: "Error handling"
type: Task
priority: High
status: todo
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-003", Title: "Error handling", Status: item.StatusTodo, Priority: item.PriorityHigh, Dependencies: []string{"TK-001"}, FilePath: path},
		{ID: "ST-001", Title: "Command parsing", Status: item.StatusTodo, Dependencies: []string{"TK-001", "TK-003"}},
		{ID: "ST-002", Title: "Profile parsing", Status: item.StatusTodo, Dependencies: []string{"TK-003", "TK-001"}},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &CompleteCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result completeJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ID != "TK-003" {
		t.Errorf("ID = %q, want TK-003", result.ID)
	}
	// Both ST-001 and ST-002 should be unblocked (all their deps are now done)
	if len(result.Unblocked) != 2 {
		t.Errorf("Unblocked = %v, want 2 items", result.Unblocked)
	}
}

func TestCompleteCmd_StillBlocked_Text(t *testing.T) {
	dir := t.TempDir()
	path := writeItemFile(t, dir, "tk-002.md", `---
id: TK-002
title: "Abstraction"
type: Task
priority: High
status: todo
dependencies: [TK-001]
---
`)
	items := []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusTodo, Dependencies: []string{"TK-001"}, FilePath: path},
		// ST-002 depends on TK-002 and TK-003 — completing TK-002 leaves it still blocked by TK-003
		{ID: "TK-003", Title: "Error handling", Status: item.StatusTodo, Dependencies: []string{"TK-001"}},
		{ID: "ST-002", Title: "Profile parsing", Status: item.StatusTodo, Dependencies: []string{"TK-002", "TK-003"}},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "TK-002"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Completed TK-002") {
		t.Errorf("expected 'Completed TK-002', got:\n%s", out)
	}
	if !strings.Contains(out, "Still blocked") {
		t.Errorf("expected 'Still blocked' section, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-002") {
		t.Errorf("expected ST-002 in still-blocked list, got:\n%s", out)
	}
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 in waiting-on, got:\n%s", out)
	}
}

func TestCompleteCmd_CustomStatuses_AlreadyDone(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "Item A", Status: "shipped"},
	}
	rc, _ := newCustomTestContext(t, items, false)

	cmd := &CompleteCmd{ID: "A"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for already-done item with custom status")
	}
}
