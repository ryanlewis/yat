package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/ryanlewis/yat/internal/item"
)

// tag returns a style that wraps text in a visible marker, so tests can tell
// which style a helper picked without depending on ANSI escape codes.
func tag(name string) lipgloss.Style {
	return lipgloss.NewStyle().Transform(func(s string) string { return name + "(" + s + ")" })
}

func TestStyles_PriorityPicksStylePerLevel(t *testing.T) {
	s := newStyles(&bytes.Buffer{}, defaultStatuses)
	s.pCritical, s.pHigh, s.pMedium, s.pLow = tag("crit"), tag("high"), tag("med"), tag("low")

	tests := []struct {
		p    item.Priority
		want string
	}{
		{item.PriorityCritical, "crit(Critical)"},
		{item.PriorityHigh, "high(High)"},
		{item.PriorityMedium, "med(Medium)"},
		{item.PriorityLow, "low(Low)"},
		{item.Priority("Urgent"), "Urgent"},
		{item.Priority(""), ""},
	}

	for _, tt := range tests {
		if got := s.Priority(tt.p); got != tt.want {
			t.Errorf("Priority(%q) = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestStyles_StatusPicksStylePerGroup(t *testing.T) {
	s := newStyles(&bytes.Buffer{}, defaultStatuses)
	s.sDone, s.sActive, s.dim = tag("done"), tag("active"), tag("dim")

	tests := []struct {
		status item.Status
		want   string
	}{
		{item.StatusDone, "done(done)"},
		{item.StatusInProgress, "active(in-progress)"},
		{item.StatusTodo, "dim(todo)"},
		{item.Status("unknown"), "dim(unknown)"},
	}

	for _, tt := range tests {
		if got := s.Status(tt.status); got != tt.want {
			t.Errorf("Status(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStyles_StatusUsesConfiguredGroups(t *testing.T) {
	s := newStyles(&bytes.Buffer{}, customStatuses())
	s.sDone, s.sActive, s.dim = tag("done"), tag("active"), tag("dim")

	for _, st := range customStatuses().Done {
		if got, want := s.Status(item.Status(st)), "done("+st+")"; got != want {
			t.Errorf("Status(%q) = %q, want %q", st, got, want)
		}
	}

	for _, st := range customStatuses().Active {
		if got, want := s.Status(item.Status(st)), "active("+st+")"; got != want {
			t.Errorf("Status(%q) = %q, want %q", st, got, want)
		}
	}
}

// ttyStyles returns Styles that take the TTY path of RenderBody while writing
// to a buffer.
func ttyStyles() *Styles {
	s := newStyles(&bytes.Buffer{}, defaultStatuses)
	s.isTTY = true

	return s
}

func TestRenderBody_NotTTYReturnsBodyUnchanged(t *testing.T) {
	s := newStyles(&bytes.Buffer{}, defaultStatuses)
	body := "# Heading\n\nSome **bold** text.\n"

	if got := s.RenderBody(body); got != body {
		t.Errorf("RenderBody = %q, want body unchanged", got)
	}
}

func TestRenderBody_TTYEmptyBody(t *testing.T) {
	if got := ttyStyles().RenderBody(""); got != "" {
		t.Errorf("RenderBody(\"\") = %q, want empty", got)
	}
}

func TestRenderBody_TTYRendersMarkdown(t *testing.T) {
	s := ttyStyles()
	body := "# Heading\n\nSome **bold** text and a [link](https://example.com).\n\n- first\n- second\n"

	got := s.RenderBody(body)

	if got == body {
		t.Fatal("RenderBody returned the raw body, want rendered markdown")
	}

	if s.glamourR == nil {
		t.Fatal("glamour renderer was not created")
	}

	for _, want := range []string{"Heading", "bold", "first", "second", "https://example.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered body missing %q:\n%s", want, got)
		}
	}

	// WithAutoStyle picks glamour's plain "notty" style when the test process
	// has no terminal, which keeps emphasis markers, so only check syntax every
	// style rewrites.
	if strings.Contains(got, "](") {
		t.Errorf("rendered body still has markdown link syntax:\n%s", got)
	}

	if strings.Contains(got, "- first") {
		t.Errorf("rendered body still has markdown list markers:\n%s", got)
	}
}

func TestRenderBody_TTYWrapsToDefaultWidth(t *testing.T) {
	body := strings.Repeat("lorem ipsum dolor ", 40) + "\n"

	got := ttyStyles().RenderBody(body)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("rendered body has %d lines, want the long paragraph wrapped:\n%s", len(lines), got)
	}

	for i, line := range lines {
		if w := lipgloss.Width(line); w > defaultTermWidth {
			t.Errorf("line %d is %d columns wide, want at most %d: %q", i, w, defaultTermWidth, line)
		}
	}

	if words := strings.Count(got, "lorem"); words != 40 {
		t.Errorf("rendered body has %d words %q, want 40", words, "lorem")
	}
}

func TestRenderBody_TTYReusesRenderer(t *testing.T) {
	s := ttyStyles()

	s.RenderBody("first")
	first := s.glamourR

	s.RenderBody("second")

	if s.glamourR != first {
		t.Error("RenderBody created a second glamour renderer, want one per Styles")
	}
}

func TestRenderBody_TTYFallsBackWithoutRenderer(t *testing.T) {
	s := ttyStyles()
	// Use up the one-time setup without creating a renderer, as happens when
	// glamour.NewTermRenderer fails.
	s.glamourOnce.Do(func() {})

	body := "# Heading\n"
	if got := s.RenderBody(body); got != body {
		t.Errorf("RenderBody = %q, want body unchanged when no renderer", got)
	}
}

func TestTermWidth_NonFileWriterUsesDefault(t *testing.T) {
	if got := termWidth(&bytes.Buffer{}); got != defaultTermWidth {
		t.Errorf("termWidth(buffer) = %d, want %d", got, defaultTermWidth)
	}
}

func TestTermWidth_NonTerminalFileUsesDefault(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = f.Close() }()

	if got := termWidth(f); got != defaultTermWidth {
		t.Errorf("termWidth(regular file) = %d, want %d", got, defaultTermWidth)
	}
}
