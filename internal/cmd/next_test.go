package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestNextCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// TK-002 and TK-003 are both High priority and same critical depth;
	// TK-003 unblocks ST-001, TK-002 unblocks nothing → TK-003 first
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 as next item, got:\n%s", out)
	}
}

func TestNextCmd_Empty(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are ready") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}

func TestNextCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &NextCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result nextJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	// TK-003 unblocks ST-001, TK-002 unblocks nothing → TK-003 first
	if result.ID != "TK-003" {
		t.Errorf("ID = %q, want TK-003", result.ID)
	}
	if result.Priority != "High" {
		t.Errorf("Priority = %q, want High", result.Priority)
	}
}
