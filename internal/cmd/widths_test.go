package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryanlewis/yat/internal/item"
)

// wideItems mixes ASCII with CJK and emoji text, which take two terminal
// columns per rune, so byte- or rune-based padding misaligns the columns.
func wideItems() []*item.Item {
	return []*item.Item{
		{ID: "TK-001", Title: "Base", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusDone},
		{ID: "TK-002", Title: "日本語のタイトル", Type: "タスク", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "TK-003", Title: "Plain title", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo},
		{ID: "TK-004", Title: "🚀 Launch", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo,
			Dependencies: []string{"TK-005"}},
		{ID: "TK-005", Title: "Café ünïcode", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo,
			Dependencies: []string{"TK-001"}},
		{ID: "TK-006", Title: "Ready but long title", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo,
			Dependencies: []string{"TK-005"}},
	}
}

// assertColumnAligned checks that marker starts at the same display column
// on every output line that contains it.
func assertColumnAligned(t *testing.T, out, marker string, wantLines int) {
	t.Helper()

	col := -1
	n := 0
	for _, line := range strings.Split(out, "\n") {
		i := strings.Index(line, marker)
		if i < 0 {
			continue
		}
		n++
		w := lipgloss.Width(line[:i])
		if col == -1 {
			col = w
		} else if w != col {
			t.Errorf("%q at column %d, want %d:\n%s", marker, w, col, out)
		}
	}
	if n != wantLines {
		t.Errorf("found %q on %d lines, want %d:\n%s", marker, n, wantLines, out)
	}
}

func TestDashboardCmd_AlignsWideTitles(t *testing.T) {
	rc, buf := newTestContext(t, wideItems(), false)
	if err := (&DashboardCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertColumnAligned(t, buf.String(), "[High]", 3)
}

func TestListCmd_AlignsWideTitles(t *testing.T) {
	rc, buf := newTestContext(t, wideItems(), false)
	if err := (&ListCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertColumnAligned(t, buf.String(), "waiting on:", 2)
}

func TestReadyCmd_AlignsWideTypes(t *testing.T) {
	rc, buf := newTestContext(t, wideItems(), false)
	if err := (&ReadyCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	// TK-002 has a wide type; titles of the other ready items must start
	// at the same column as its title.
	want, found := -1, 0
	for _, line := range strings.Split(out, "\n") {
		for _, title := range []string{"日本語のタイトル", "Plain title", "Café ünïcode"} {
			if i := strings.Index(line, title); i >= 0 {
				found++
				w := lipgloss.Width(line[:i])
				if want == -1 {
					want = w
				} else if w != want {
					t.Errorf("title %q at column %d, want %d:\n%s", title, w, want, out)
				}
			}
		}
	}
	if found != 3 {
		t.Errorf("found %d titles, want 3:\n%s", found, out)
	}
}

func TestStatusCmd_AlignsWidePhases(t *testing.T) {
	items := []*item.Item{
		{ID: "TK-001", Title: "One", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo, Phase: "フェーズ1"},
		{ID: "TK-002", Title: "Two", Type: "Task", Priority: item.PriorityHigh, Status: item.StatusTodo, Phase: "alpha"},
	}
	rc, buf := newTestContext(t, items, false)
	if err := (&StatusCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertColumnAligned(t, buf.String(), "1  (0 pts)", 2)
}

// TestTabularOutput_TitlesWithBreaks checks that newlines and tabs in titles
// are shown as spaces, so each item stays on one line with its columns intact.
func TestTabularOutput_TitlesWithBreaks(t *testing.T) {
	broken := func() []*item.Item {
		items := wideItems()
		items[2].Title = "Plain\ntitle"
		items[5].Title = "Ready\tbut\r\nlong title"
		return items
	}

	for _, tc := range []struct {
		name   string
		cmd    interface{ Run(*RunContext) error }
		marker string
		lines  int
	}{
		{"dashboard", &DashboardCmd{}, "[High]", 3},
		{"list", &ListCmd{}, "waiting on:", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, buf := newTestContext(t, broken(), false)
			if err := tc.cmd.Run(rc); err != nil {
				t.Fatalf("Run: %v", err)
			}
			out := buf.String()
			if strings.Contains(out, "\t") {
				t.Errorf("tab in output:\n%s", out)
			}
			assertColumnAligned(t, out, tc.marker, tc.lines)
		})
	}

	rc, buf := newTestContext(t, broken(), false)
	if err := (&ReadyCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), "Plain title\n") {
		t.Errorf("expected title on one line, got:\n%s", buf.String())
	}

	rc, buf = newTestContext(t, broken(), true)
	if err := (&ListCmd{}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(buf.String(), `"Plain\ntitle"`) {
		t.Errorf("expected raw title in JSON, got:\n%s", buf.String())
	}
}

func TestOneLine_VerticalWhitespace(t *testing.T) {
	in := "a\vb\fc\u0085d e f\r\ng\th"
	if got, want := oneLine(in), "a b c d e f g h"; got != want {
		t.Errorf("oneLine(%q) = %q, want %q", in, got, want)
	}
}

// TestSingleItemOutput_TitlesWithBreaks checks that blocked, show, next,
// start and complete print a title with line breaks on one line.
func TestSingleItemOutput_TitlesWithBreaks(t *testing.T) {
	broken := func(t *testing.T) []*item.Item {
		dir := t.TempDir()
		return []*item.Item{
			{ID: "A", Title: "First\nitem", Status: item.StatusTodo,
				FilePath: writeItemFile(t, dir, "a.md", "---\nid: A\ntitle: x\nstatus: todo\n---\n")},
			{ID: "B", Title: "Second\r\nitem", Status: item.StatusTodo, Dependencies: []string{"A"}},
			{ID: "C", Title: "Third item", Status: item.StatusTodo, Dependencies: []string{"A", "D"}},
			{ID: "D", Title: "Fourth", Status: item.StatusTodo},
		}
	}

	for _, tc := range []struct {
		name string
		cmd  interface{ Run(*RunContext) error }
		want []string
	}{
		{"blocked", &BlockedCmd{}, []string{"B  Second item\n", "C  Third item\n"}},
		{"show", &ShowCmd{ID: "A"}, []string{"A: First item\n"}},
		{"next", &NextCmd{}, []string{"A: First item\n"}},
		{"start", &StartCmd{ID: "A"}, []string{"Started A: First item\n"}},
		{"complete", &CompleteCmd{ID: "A"}, []string{
			"Completed A: First item\n", "B  Second item\n", "C  Third item (waiting on: D)\n",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc, buf := newTestContext(t, broken(t), false)
			if err := tc.cmd.Run(rc); err != nil {
				t.Fatalf("Run: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(buf.String(), w) {
					t.Errorf("expected %q in output, got:\n%s", w, buf.String())
				}
			}
		})
	}
}
