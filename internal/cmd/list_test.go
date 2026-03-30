package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestListCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// Should have group headers
	if !strings.Contains(out, "READY") {
		t.Errorf("expected READY header, got:\n%s", out)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Errorf("expected BLOCKED header, got:\n%s", out)
	}
	if !strings.Contains(out, "DONE") {
		t.Errorf("expected DONE header, got:\n%s", out)
	}
	// Ready items: TK-002 and TK-003 (all deps done, not done themselves)
	if !strings.Contains(out, "TK-002") || !strings.Contains(out, "TK-003") {
		t.Errorf("expected ready items TK-002 and TK-003, got:\n%s", out)
	}
	// Blocked items: ST-001 and ST-002
	if !strings.Contains(out, "ST-001") || !strings.Contains(out, "ST-002") {
		t.Errorf("expected blocked items ST-001 and ST-002, got:\n%s", out)
	}
	// Blocked items should show waiting on
	if !strings.Contains(out, "waiting on:") {
		t.Errorf("expected 'waiting on:' for blocked items, got:\n%s", out)
	}
	// Done items: SP-001 and TK-001
	if !strings.Contains(out, "SP-001") || !strings.Contains(out, "TK-001") {
		t.Errorf("expected done items SP-001 and TK-001, got:\n%s", out)
	}
}

func TestListCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []listItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 6 {
		t.Fatalf("expected 6 items, got %d", len(result))
	}

	// Count by group
	groups := make(map[string]int)
	for _, r := range result {
		groups[r.Group]++
	}
	if groups["ready"] != 2 {
		t.Errorf("ready count = %d, want 2", groups["ready"])
	}
	if groups["blocked"] != 2 {
		t.Errorf("blocked count = %d, want 2", groups["blocked"])
	}
	if groups["done"] != 2 {
		t.Errorf("done count = %d, want 2", groups["done"])
	}
}

func TestListCmd_WithActive(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ACTIVE") {
		t.Errorf("expected ACTIVE header, got:\n%s", out)
	}
}

func TestListCmd_Empty(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, false)

	cmd := &ListCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items found") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

func TestListCmd_PhaseFilter(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "1", Status: item.StatusTodo},
		{ID: "C", Phase: "2", Status: item.StatusTodo},
		{ID: "D", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{Phase: "1"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Errorf("expected phase 1 items A and B, got:\n%s", out)
	}
	if strings.Contains(out, " C") || strings.Contains(out, " D") {
		t.Errorf("should not contain items from other phases, got:\n%s", out)
	}
}

func TestListCmd_PhaseActive(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusDone},
		{ID: "B", Phase: "2", Status: item.StatusTodo},
		{ID: "C", Phase: "2", Status: item.StatusInProgress},
		{ID: "D", Phase: "3", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, false)

	// "active" should resolve to phase "2" (earliest incomplete)
	cmd := &ListCmd{Phase: "active"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "B") || !strings.Contains(out, "C") {
		t.Errorf("expected phase 2 items B and C, got:\n%s", out)
	}
	if strings.Contains(out, " A") || strings.Contains(out, " D") {
		t.Errorf("should not contain items from other phases, got:\n%s", out)
	}
}

func TestListCmd_PhaseFilter_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusTodo},
		{ID: "B", Phase: "2", Status: item.StatusTodo},
		{ID: "C", Phase: "2", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ListCmd{Phase: "2"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []listItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result))
	}
	for _, r := range result {
		if r.ID != "B" && r.ID != "C" {
			t.Errorf("unexpected item %s in phase 2 filter", r.ID)
		}
	}
}

func TestListCmd_PhaseNoMatches(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Phase: "1", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{Phase: "99"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items found") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

func TestListCmd_PhaseActiveNoPhases(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
		{ID: "B", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ListCmd{Phase: "active"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No active phase") {
		t.Errorf("expected 'No active phase' message, got:\n%s", buf.String())
	}
}

func TestListCmd_PhaseActiveNoPhases_JSON(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, true)

	cmd := &ListCmd{Phase: "active"}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []listItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}
