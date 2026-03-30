package item

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantID   string
		wantNil  bool
		wantErr  bool
		wantBody string
	}{
		{
			name: "valid frontmatter",
			input: `---
id: TK-001
title: "Test task"
type: Task
priority: Critical
points: 3
dependencies: [TK-002]
status: todo
phase: "1"
---

## Body content
Some text here.
`,
			wantID:   "TK-001",
			wantBody: "\n## Body content\nSome text here.\n",
		},
		{
			name: "no id field returns nil",
			input: `---
title: "No ID"
---

Body text.
`,
			wantNil: true,
		},
		{
			name:    "no frontmatter returns nil",
			input:   "# Just a regular markdown file\n\nNo frontmatter here.\n",
			wantNil: true,
		},
		{
			name: "empty dependencies",
			input: `---
id: SP-001
title: "Spike"
type: Spike
priority: High
points: 2
dependencies: []
status: done
phase: "0"
---
`,
			wantID: "SP-001",
		},
		{
			name: "no closing delimiter returns nil",
			input: `---
id: TK-001
title: "Unclosed"
`,
			wantNil: true,
		},
		{
			name:   "CRLF line endings",
			input:  "---\r\nid: TK-001\r\ntitle: \"CRLF test\"\r\nstatus: todo\r\n---\r\n\r\nBody.\r\n",
			wantID: "TK-001",
		},
		{
			name:    "opening delimiter not followed by newline",
			input:   "---yaml\nid: TK-001\n---\n",
			wantNil: true,
		},
		{
			name:    "opening delimiter with trailing text",
			input:   "--- title\nid: TK-001\n---\n",
			wantNil: true,
		},
		{
			name:    "invalid status",
			input:   "---\nid: TK-001\nstatus: banana\n---\n",
			wantErr: true,
		},
		{
			name:    "invalid priority",
			input:   "---\nid: TK-001\npriority: critical\n---\n",
			wantErr: true,
		},
		{
			name:    "item-like frontmatter without id",
			input:   "---\ntype: Task\nstatus: todo\n---\n\nBody.\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse([]byte(tt.input), "test.md")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if item != nil {
					t.Fatalf("expected nil, got %+v", item)
				}
				return
			}

			if item == nil {
				t.Fatal("expected non-nil item")
			}

			if item.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", item.ID, tt.wantID)
			}

			if tt.wantBody != "" && item.Body != tt.wantBody {
				t.Errorf("Body = %q, want %q", item.Body, tt.wantBody)
			}
		})
	}
}

func TestParseWithOptions_Aliases(t *testing.T) {
	input := "---\nid: TK-001\ntitle: \"Test\"\nstate: backlog\n---\n\nBody.\n"
	opts := ParseOptions{
		Aliases:       map[string]string{"state": "status"},
		ValidStatuses: NewStatusSet([]string{"backlog", "in-progress", "done"}),
		DefaultStatus: "backlog",
	}

	item, err := ParseWithOptions([]byte(input), "test.md", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.Status != "backlog" {
		t.Errorf("Status = %q, want backlog", item.Status)
	}
}

func TestParseWithOptions_CustomStatusValidation(t *testing.T) {
	input := "---\nid: TK-001\nstatus: review\n---\n"
	opts := ParseOptions{
		ValidStatuses: NewStatusSet([]string{"backlog", "review", "shipped"}),
		DefaultStatus: "backlog",
	}

	item, err := ParseWithOptions([]byte(input), "test.md", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != "review" {
		t.Errorf("Status = %q, want review", item.Status)
	}
}

func TestParseWithOptions_CustomStatusRejected(t *testing.T) {
	input := "---\nid: TK-001\nstatus: banana\n---\n"
	opts := ParseOptions{
		ValidStatuses: NewStatusSet([]string{"backlog", "review", "shipped"}),
	}

	_, err := ParseWithOptions([]byte(input), "test.md", opts)
	if err == nil {
		t.Fatal("expected error for invalid custom status")
	}
}

func TestParseWithOptions_DefaultStatusApplied(t *testing.T) {
	input := "---\nid: TK-001\ntitle: \"No status\"\n---\n\nBody.\n"
	opts := ParseOptions{
		ValidStatuses: NewStatusSet([]string{"backlog", "done"}),
		DefaultStatus: "backlog",
	}

	item, err := ParseWithOptions([]byte(input), "test.md", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != "backlog" {
		t.Errorf("Status = %q, want backlog", item.Status)
	}
}

func TestParseWithOptions_BackwardCompat(t *testing.T) {
	input := "---\nid: TK-001\nstatus: todo\n---\n"
	item, err := ParseWithOptions([]byte(input), "test.md", ParseOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Status != StatusTodo {
		t.Errorf("Status = %q, want todo", item.Status)
	}
}

func TestParse_EmptyStatusNormalizedToTodo(t *testing.T) {
	input := "---\nid: TK-001\ntitle: \"No status\"\n---\n\nBody.\n"
	item, err := Parse([]byte(input), "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.Status != StatusTodo {
		t.Errorf("Status = %q, want %q", item.Status, StatusTodo)
	}
}

func TestParseFields(t *testing.T) {
	input := `---
id: ST-001
title: "CLI command parsing"
type: Story
priority: Critical
points: 5
dependencies: [TK-001, TK-003]
status: in-progress
phase: "2"
---

Body.
`
	item, err := Parse([]byte(input), "test.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.Title != "CLI command parsing" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.Type != "Story" {
		t.Errorf("Type = %q", item.Type)
	}
	if item.Priority != PriorityCritical {
		t.Errorf("Priority = %q", item.Priority)
	}
	if item.Points != 5 {
		t.Errorf("Points = %d", item.Points)
	}
	if len(item.Dependencies) != 2 || item.Dependencies[0] != "TK-001" || item.Dependencies[1] != "TK-003" {
		t.Errorf("Dependencies = %v", item.Dependencies)
	}
	if item.Status != StatusInProgress {
		t.Errorf("Status = %q", item.Status)
	}
	if item.Phase != "2" {
		t.Errorf("Phase = %q", item.Phase)
	}
	if item.FilePath != "test.md" {
		t.Errorf("FilePath = %q", item.FilePath)
	}
}

// --- extractHeading ---

func TestExtractHeading(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"h1", "# Hello", "Hello"},
		{"h2", "## Hello", "Hello"},
		{"h3", "### Hello", "Hello"},
		{"h4 ignored", "#### Hello", ""},
		{"h1 with whitespace", "#  Spaced Out  ", "Spaced Out"},
		{"no heading", "Just some text\nMore text", ""},
		{"empty body", "", ""},
		{"first heading wins", "## Second\n# First", "Second"},
		{"text before heading", "Some intro text\n\n# Title", "Title"},
		{"not a heading without space", "#NoSpace", ""},
		{"heading inside code block", "```\n# Not a heading\n```\n## Real Title", "Real Title"},
		{"heading inside code block with lang", "```python\n# Comment\n```\n# Title", "Title"},
		{"unclosed code block hides all", "```\n# Hidden", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHeading([]byte(tt.body))
			if got != tt.want {
				t.Errorf("extractHeading() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse_TitleFromBody(t *testing.T) {
	data := []byte("---\nid: TK-001\nstatus: todo\n---\n# My Title\n\nBody content.\n")
	item, err := Parse(data, "test.md")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if item.Title != "My Title" {
		t.Errorf("Title = %q, want My Title", item.Title)
	}
}

func TestParse_FrontmatterTitleTakesPrecedence(t *testing.T) {
	data := []byte("---\nid: TK-001\ntitle: Explicit\nstatus: todo\n---\n# Body Title\n")
	item, err := Parse(data, "test.md")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if item.Title != "Explicit" {
		t.Errorf("Title = %q, want Explicit", item.Title)
	}
}
