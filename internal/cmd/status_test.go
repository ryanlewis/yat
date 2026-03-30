package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestStatusCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "6 items") {
		t.Errorf("expected '6 items' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "done:") {
		t.Errorf("expected 'done:' in output, got:\n%s", out)
	}
}

func TestStatusCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if result.TotalItems != 6 {
		t.Errorf("TotalItems = %d, want 6", result.TotalItems)
	}
	if result.ByStatus["done"].Count != 2 {
		t.Errorf("ByStatus[done].Count = %d, want 2", result.ByStatus["done"].Count)
	}
	if result.ByStatus["initial"].Count != 4 {
		t.Errorf("ByStatus[initial].Count = %d, want 4", result.ByStatus["initial"].Count)
	}
}

func TestStatusCmd_CustomStatuses(t *testing.T) {
	items := []*item.Item{
		{ID: "A", Status: "shipped", Type: "Task", Points: 3},
		{ID: "B", Status: "review", Type: "Task", Points: 5},
		{ID: "C", Status: "backlog", Type: "Task", Points: 2},
	}
	rc, buf := newCustomTestContext(t, items, true)

	cmd := &StatusCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result statusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if result.ByStatus["done"].Count != 1 {
		t.Errorf("done count = %d, want 1", result.ByStatus["done"].Count)
	}
	if result.ByStatus["active"].Count != 1 {
		t.Errorf("active count = %d, want 1", result.ByStatus["active"].Count)
	}
	if result.ByStatus["initial"].Count != 1 {
		t.Errorf("initial count = %d, want 1", result.ByStatus["initial"].Count)
	}
}

func TestPluralizeType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Task", "tasks"},
		{"Story", "stories"},
		{"Spike", "spikes"},
	}

	for _, tt := range tests {
		got := pluralizeType(tt.input)
		if got != tt.want {
			t.Errorf("pluralizeType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
