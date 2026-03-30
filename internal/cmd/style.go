package cmd

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/ryanlewis/yat/internal/config"
	"github.com/ryanlewis/yat/internal/item"
	"golang.org/x/term"
)

// Styles provides terminal-aware text styling for CLI output.
// When output is not a TTY, all methods return unstyled text.
type Styles struct {
	w     io.Writer
	isTTY bool

	glamourOnce sync.Once
	glamourR    *glamour.TermRenderer

	bold      lipgloss.Style
	dim       lipgloss.Style
	pCritical lipgloss.Style
	pHigh     lipgloss.Style
	pMedium   lipgloss.Style
	pLow      lipgloss.Style
	sDone     lipgloss.Style
	sActive   lipgloss.Style

	statuses config.StatusGroups
}

const defaultTermWidth = 80

func newStyles(w io.Writer, statuses config.StatusGroups) *Styles {
	r := lipgloss.NewRenderer(w)

	return &Styles{
		w:     w,
		isTTY: isTermWriter(w),
		bold:  r.NewStyle().Bold(true),
		dim:   r.NewStyle().Faint(true),
		pCritical: r.NewStyle().Bold(true).
			Foreground(lipgloss.Color("1")), // red
		pHigh: r.NewStyle().
			Foreground(lipgloss.Color("3")), // yellow
		pMedium: r.NewStyle().
			Foreground(lipgloss.Color("6")), // cyan
		pLow: r.NewStyle().
			Foreground(lipgloss.Color("2")), // green
		sDone: r.NewStyle().
			Foreground(lipgloss.Color("2")), // green
		sActive: r.NewStyle().
			Foreground(lipgloss.Color("3")), // yellow
		statuses: statuses,
	}
}

// Header styles an "ID: Title" header line.
func (s *Styles) Header(id, title string) string {
	return s.bold.Render(id+": "+title) + "\n"
}

// GroupHeader styles a section header like "READY" or "BLOCKED".
func (s *Styles) GroupHeader(label string) string {
	return s.bold.Render(strings.ToUpper(label)) + "\n"
}

// Priority styles a priority value with the appropriate color.
func (s *Styles) Priority(p item.Priority) string {
	switch p {
	case item.PriorityCritical:
		return s.pCritical.Render(string(p))
	case item.PriorityHigh:
		return s.pHigh.Render(string(p))
	case item.PriorityMedium:
		return s.pMedium.Render(string(p))
	case item.PriorityLow:
		return s.pLow.Render(string(p))
	default:
		return string(p)
	}
}

// Status styles a status value based on its semantic group.
func (s *Styles) Status(status item.Status) string {
	st := string(status)

	switch {
	case s.statuses.IsDone(st):
		return s.sDone.Render(st)
	case s.statuses.IsActive(st):
		return s.sActive.Render(st)
	default:
		return s.dim.Render(st)
	}
}

// Bold renders text in bold.
func (s *Styles) Bold(text string) string {
	return s.bold.Render(text)
}

// Dim renders text in faint/dim style.
func (s *Styles) Dim(text string) string {
	return s.dim.Render(text)
}

// RenderBody renders a markdown body for terminal display.
// When not a TTY, returns the body as-is.
func (s *Styles) RenderBody(body string) string {
	if !s.isTTY || body == "" {
		return body
	}

	s.glamourOnce.Do(func() {
		width := termWidth(s.w)
		gr, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
		if err == nil {
			s.glamourR = gr
		}
	})

	if s.glamourR == nil {
		return body
	}

	rendered, err := s.glamourR.Render(body)
	if err != nil {
		return body
	}

	return rendered
}

func isTermWriter(w interface{}) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors fit in int
}

func termWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return defaultTermWidth
	}

	width, _, err := term.GetSize(int(f.Fd())) //nolint:gosec // file descriptors fit in int
	if err != nil || width <= 0 {
		return defaultTermWidth
	}

	return width
}
