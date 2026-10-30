package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
	"github.com/ryanlewis/yat/internal/item"
)

const readBackData = "---\nid: TK-001\ntitle: Title\ntype: Task\npriority: High\npoints: 3\n" +
	"dependencies: [TK-002, TK-006]\nstatus: todo\nphase: p1\n---\n"

// readBackItem returns the item readBackData describes.
func readBackItem() *item.Item {
	return &item.Item{
		ID:           "TK-001",
		Title:        "Title",
		Type:         "Task",
		Priority:     item.PriorityHigh,
		Points:       3,
		Dependencies: []string{"TK-002", "TK-006"},
		Status:       "todo",
		Phase:        "p1",
		FilePath:     "TK-001-title.md",
	}
}

func TestCheckReadsBack(t *testing.T) {
	rc, _ := newAddContext(t, addTestDir(t), defaultConfig(), false)

	tests := []struct {
		name   string
		data   string
		change func(*item.Item)
		ok     bool
	}{
		{name: "same fields", data: readBackData, ok: true},
		{name: "id differs", data: readBackData, change: func(it *item.Item) { it.ID = "TK-009" }},
		{name: "title differs", data: readBackData, change: func(it *item.Item) { it.Title = "Other" }},
		{name: "type differs", data: readBackData, change: func(it *item.Item) { it.Type = "Story" }},
		{name: "priority differs", data: readBackData, change: func(it *item.Item) { it.Priority = item.PriorityLow }},
		{name: "points differ", data: readBackData, change: func(it *item.Item) { it.Points = 5 }},
		{name: "status differs", data: readBackData, change: func(it *item.Item) { it.Status = "in-progress" }},
		{name: "phase differs", data: readBackData, change: func(it *item.Item) { it.Phase = "p2" }},
		{
			name:   "dependencies in another order",
			data:   readBackData,
			change: func(it *item.Item) { it.Dependencies = []string{"TK-006", "TK-002"} },
		},
		{
			name:   "dependency missing",
			data:   readBackData,
			change: func(it *item.Item) { it.Dependencies = []string{"TK-002"} },
		},
		{name: "no frontmatter", data: "# Title\n"},
		{name: "ignored", data: strings.Replace(readBackData, "phase: p1\n", "phase: p1\nyat: ignore\n", 1)},
		{name: "parse error", data: strings.Replace(readBackData, "points: 3", "points: three", 1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := readBackItem()
			if tt.change != nil {
				tt.change(it)
			}

			err := rc.checkReadsBack([]byte(tt.data), it)
			if tt.ok {
				if err != nil {
					t.Errorf("checkReadsBack: %v", err)
				}

				return
			}

			want := "cannot write TK-001-title.md: it would not read back as written under this project's field_aliases"
			if err == nil || err.Error() != want {
				t.Errorf("checkReadsBack error = %v, want %q", err, want)
			}
		})
	}
}

// The check reads data under the project's field_aliases and statuses, so an
// aliased file with a custom status reads back.
func TestCheckReadsBack_AliasesAndCustomStatus(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "TK-001.md", "---\nkey: TK-001\nstate: backlog\n---\n")

	cfg := &config.Config{
		Statuses:     customStatuses(),
		FieldAliases: map[string]string{"id": "key", "status": "state"},
	}
	rc, _ := newAddContext(t, dir, cfg, false)

	it := &item.Item{ID: "TK-002", Title: "Aliased", Status: "backlog", FilePath: "TK-002-aliased.md"}

	if err := rc.checkReadsBack([]byte("---\nkey: TK-002\ntitle: Aliased\nstate: backlog\n---\n"), it); err != nil {
		t.Errorf("checkReadsBack: %v", err)
	}
}

// field_aliases can put a field under a key that stops the file reading back.
// add refuses to write such a file.
func TestAddCmd_RefusesFileThatWouldNotReadBack(t *testing.T) {
	tests := []struct {
		name  string
		alias string
		title string
		file  string
	}{
		// title: ignore under the yat key makes yat skip the file.
		{name: "yat directive", alias: "yat", title: "ignore", file: "TK-007-ignore.md"},
		// A ~ key decodes as null, so the frontmatter does not parse.
		{name: "null key", alias: "~", title: "x", file: "TK-007-x.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := addTestDir(t)
			cfg := defaultConfig()
			cfg.FieldAliases = map[string]string{"title": tt.alias}
			rc, _ := newAddContext(t, dir, cfg, false)

			err := (&AddCmd{Title: tt.title, Type: "Task", stdin: strings.NewReader("")}).Run(rc)

			path := filepath.Join(dir, "tasks", tt.file)
			if err == nil || !strings.Contains(err.Error(), "cannot write "+path+": it would not read back") {
				t.Errorf("expected a read-back error for %s, got: %v", path, err)
			}

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("%s was written (stat: %v)", path, err)
			}
		})
	}
}
