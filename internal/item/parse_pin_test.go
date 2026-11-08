package item

import (
	"reflect"
	"strings"
	"testing"
)

// These tests pin parser branches that the other parse tests cover but do not
// assert on, so a refactor that changes their outcome fails a test.

func TestExtractHeading_ListPins(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"heading straight after a list item", "- a\n# T", "T"},
		{"star marker opens a list item", "* a\n    # T", "T"},
		{"plus marker opens a list item", "+ a\n    # T", "T"},
		{"ordered item not at 1 opens a list after a blank", "2. a\n    # T", "T"},
		{"ordered item not at 1 interrupts a list paragraph", "- a\n10. b\n      # T", "T"},
		{"empty item has no paragraph to continue", "-\ntext\n    # T", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractHeading([]byte(tt.body)); got != tt.want {
				t.Errorf("extractHeading(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestListMarker_Pins(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"", 0},
		{"-", 1},
		{"* a", 1},
		{"+ a", 1},
		{"1. a", 2},
		{"1) a", 2},
		{"0.", 2},
		{"9.", 2},
		{"123456789. a", 10},
		{"1234567890. a", 0},
		{". a", 0},
		{"1", 0},
		{"12x", 0},
		{"a", 0},
	}

	for _, tt := range tests {
		if got := listMarker([]byte(tt.line)); got != tt.want {
			t.Errorf("listMarker(%q) = %d, want %d", tt.line, got, tt.want)
		}
	}
}

func TestListContentColumn_Pins(t *testing.T) {
	tests := []struct {
		line  string
		start int
		want  int
	}{
		{"- a", 0, 2},
		{"-", 0, 2},
		{"-", 3, 5},
		{"-\ta", 0, 4},
		{"-    a", 0, 5},
		{"-     a", 0, 2},
		{"- - -", 0, 0},
		{"-a", 0, 0},
		{"a", 0, 0},
	}

	for _, tt := range tests {
		if got := listContentColumn([]byte(tt.line), tt.start); got != tt.want {
			t.Errorf("listContentColumn(%q, %d) = %d, want %d", tt.line, tt.start, got, tt.want)
		}
	}
}

func TestFrontmatterBounds_Pins(t *testing.T) {
	tests := []struct {
		data                      string
		fmStart, fmEnd, bodyStart int
		ok                        bool
	}{
		{"---", 0, 0, 0, false},
		{"---\na: 1\n---\r\nbody", 3, 8, 14, true},
		{"---\na: 1\n---\rbody", 3, 8, 13, true},
		{"---\na: 1\n---", 3, 8, 12, true},
	}

	for _, tt := range tests {
		fmStart, fmEnd, bodyStart, ok := frontmatterBounds([]byte(tt.data))
		if fmStart != tt.fmStart || fmEnd != tt.fmEnd || bodyStart != tt.bodyStart || ok != tt.ok {
			t.Errorf("frontmatterBounds(%q) = %d, %d, %d, %v, want %d, %d, %d, %v", tt.data,
				fmStart, fmEnd, bodyStart, ok, tt.fmStart, tt.fmEnd, tt.bodyStart, tt.ok)
		}
	}
}

func TestParse_CRLFAfterClosingDelimiter(t *testing.T) {
	item, err := Parse([]byte("---\nid: A\n---\r\nbody"), "a.md")
	if err != nil || item == nil {
		t.Fatalf("Parse() = %v, %v", item, err)
	}

	if item.Body != "body" {
		t.Errorf("Body = %q, want %q", item.Body, "body")
	}
}

func TestParse_YatDirectiveIsNotAnExtraField(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]any
	}{
		{"---\nid: A\nyat: notrack\n---\n", nil},
		{"---\nid: A\nyat: notrack\nfoo: 1\n---\n", map[string]any{"foo": 1}},
	}

	for _, tt := range tests {
		item, err := Parse([]byte(tt.input), "a.md")
		if err != nil || item == nil {
			t.Fatalf("Parse(%q) = %v, %v", tt.input, item, err)
		}

		if !reflect.DeepEqual(item.Extra, tt.want) {
			t.Errorf("Parse(%q).Extra = %#v, want %#v", tt.input, item.Extra, tt.want)
		}
	}
}

func TestParseWithOptions_InvalidYAMLWithAliases(t *testing.T) {
	opts := ParseOptions{Aliases: map[string]string{"state": "status"}}

	item, err := ParseWithOptions([]byte("---\nid: [\n---\n"), "a.md", opts)
	if item != nil || err == nil || !strings.HasPrefix(err.Error(), "parsing frontmatter in a.md: ") {
		t.Fatalf("ParseWithOptions() = %v, %v, want a parsing error", item, err)
	}
}

func TestParseWithOptions_IdentityAliasOwnsItsField(t *testing.T) {
	opts := ParseOptions{Aliases: map[string]string{"status": "status", "state": "status"}}

	item, err := ParseWithOptions([]byte("---\nid: A\nstatus: todo\nstate: done\n---\n"), "a.md", opts)
	if err != nil || item == nil {
		t.Fatalf("ParseWithOptions() = %v, %v", item, err)
	}

	if item.Status != StatusTodo || item.StatusKey != statusField {
		t.Errorf("Status, StatusKey = %q, %q, want todo, status", item.Status, item.StatusKey)
	}

	if want := map[string]any{"state": "done"}; !reflect.DeepEqual(item.Extra, want) {
		t.Errorf("Extra = %#v, want %#v", item.Extra, want)
	}
}
