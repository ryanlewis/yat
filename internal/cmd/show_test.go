package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestShowCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "TK-003"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-003") || !strings.Contains(out, "Error handling") {
		t.Errorf("expected TK-003 details, got:\n%s", out)
	}
}

func TestShowCmd_NotFound(t *testing.T) {
	items := makeTestItems()
	rc, _ := newTestContext(t, items, false)

	cmd := &ShowCmd{ID: "NONEXISTENT"}
	err := cmd.Run(rc)
	if err == nil {
		t.Fatal("expected error for unknown ID")
	}
}

func TestShowCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ShowCmd{ID: "ST-001"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if result.ID != "ST-001" {
		t.Errorf("ID = %q, want ST-001", result.ID)
	}
	if len(result.Dependencies) != 2 {
		t.Errorf("Dependencies = %v, want 2 items", result.Dependencies)
	}
}

func TestShowCmd_DuplicateDependencies(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Title: "First", Status: item.StatusTodo},
		{ID: "B", Title: "Second", Status: item.StatusTodo, Dependencies: []string{"A", "A"}},
	}

	rc, buf := newTestContext(t, items, false)
	if err := (&ShowCmd{ID: "B"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if strings.Contains(buf.String(), "A A") {
		t.Errorf("dependency listed twice:\n%s", buf.String())
	}

	rc, buf = newTestContext(t, items, true)
	if err := (&ShowCmd{ID: "B"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result showJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result.Dependencies) != 1 || result.Dependencies[0] != "A" {
		t.Errorf("Dependencies = %v, want [A]", result.Dependencies)
	}
}
