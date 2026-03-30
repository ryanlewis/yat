package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestBlockedCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	// ST-001 and ST-002 are blocked (their deps aren't all done)
	if !strings.Contains(out, "ST-001") {
		t.Errorf("expected ST-001 in blocked output, got:\n%s", out)
	}
	if !strings.Contains(out, "ST-002") {
		t.Errorf("expected ST-002 in blocked output, got:\n%s", out)
	}
}

func TestBlockedCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []blockedItemJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 blocked items, got %d", len(result))
	}

	// Each blocked item should have non-empty WaitingOn
	for _, item := range result {
		if len(item.WaitingOn) == 0 {
			t.Errorf("blocked item %s has empty WaitingOn", item.ID)
		}
	}
}

func TestBlockedCmd_NoneBlocked(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusTodo},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &BlockedCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are blocked") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}
