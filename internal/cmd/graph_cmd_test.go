package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestGraphCmd_Text(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, false)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Layer 0") {
		t.Errorf("expected 'Layer 0' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "\u2192") {
		t.Errorf("expected edges in output, got:\n%s", out)
	}
	if !strings.Contains(out, "\u2705") {
		t.Errorf("expected done emoji in output, got:\n%s", out)
	}
}

func TestGraphCmd_JSON(t *testing.T) {
	items := makeTestItems()
	rc, buf := newTestContext(t, items, true)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var result graphJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result.Layers) < 3 {
		t.Errorf("expected at least 3 layers, got %d", len(result.Layers))
	}
	if len(result.Edges) == 0 {
		t.Error("expected edges in graph JSON")
	}
}

func TestGraphCmd_Empty(t *testing.T) {
	rc, buf := newTestContext(t, []*item.Item{}, false)

	cmd := &GraphCmd{}
	if err := cmd.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "No items found") {
		t.Errorf("expected empty message, got:\n%s", buf.String())
	}
}
