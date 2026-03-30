package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestReadyCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-002") {
		t.Errorf("expected TK-002 in ready output, got:\n%s", out)
	}
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 in ready output, got:\n%s", out)
	}
}

func TestReadyCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result []readyJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 ready items, got %d", len(result))
	}
}

func TestReadyCmd_ExcludesActive(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003 is in-progress, deps done
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 excluded from ready (in-progress), got:\n%s", out)
	}
	if !strings.Contains(out, "TK-002") {
		t.Errorf("expected TK-002 in ready output, got:\n%s", out)
	}
}

func TestReadyCmd_AllFlag(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress // TK-003 is in-progress, deps done
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{All: true}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TK-003") {
		t.Errorf("expected TK-003 in ready --all output, got:\n%s", out)
	}
	if !strings.Contains(out, "TK-002") {
		t.Errorf("expected TK-002 in ready --all output, got:\n%s", out)
	}
}

func TestReadyCmd_Empty(t *testing.T) {
	// All items done
	items := []*item.Item{
		{ID: "TK-001", Status: item.StatusDone},
	}
	rc, buf := newTestContext(t, items, false)

	cmd := &ReadyCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items are ready") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}
