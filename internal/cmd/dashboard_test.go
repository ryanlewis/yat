package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

func TestDashboardCmd_Text(t *testing.T) {
	items := makeTestItems()
	items[3].Status = item.StatusInProgress
	rc, buf := newTestContext(t, items, false)
	rc.Dir = defaultDir

	if err := (&DashboardCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"spec/ · 6 items (1 in progress, 1 ready, 2 blocked, 2 done)",
		"In progress:\n  TK-003",
		"Up next (ready):\n  TK-002",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "more") || strings.Contains(out, "Phase:") {
		t.Errorf("unexpected overflow or phase line, got:\n%s", out)
	}
}

func TestDashboardCmd_ReadyLimit(t *testing.T) {
	items := make([]*item.Item, 0, dashboardReadyLimit+3)
	for i := range dashboardReadyLimit + 3 {
		items = append(items, &item.Item{
			ID: fmt.Sprintf("T-%02d", i), Title: "t", Status: item.StatusTodo, Phase: "mvp", Points: 2,
		})
	}
	items[0].Status = item.StatusDone
	rc, buf := newTestContext(t, items, true)

	if err := (&DashboardCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got dashboardJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if len(got.Ready) != dashboardReadyLimit || got.ReadyMore != 2 || got.Counts.Ready != dashboardReadyLimit+2 {
		t.Errorf("ready = %d, more = %d, count = %d", len(got.Ready), got.ReadyMore, got.Counts.Ready)
	}
	if got.Phase == nil || got.Phase.Name != "mvp" || got.Phase.DonePoints != 2 || got.Phase.TotalPoints != 16 {
		t.Errorf("phase = %+v", got.Phase)
	}

	rc.JSON = false
	buf.Reset()
	if err := (&DashboardCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"...and 2 more — run `yat ready`", "Phase: mvp * (2/16 pts done)"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("expected %q in output, got:\n%s", want, buf.String())
		}
	}
}

func TestWriteNoProject(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteNoProject(&buf, false, defaultDir); err != nil {
		t.Fatalf("WriteNoProject: %v", err)
	}
	if !strings.Contains(buf.String(), "yat init") {
		t.Errorf("expected yat init hint, got: %s", buf.String())
	}
}
