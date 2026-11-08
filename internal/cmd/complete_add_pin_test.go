package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/item"
)

// completeFixture holds TK-002 on disk; completing it unblocks ST-001 and
// leaves ST-002 waiting on TK-003.
func completeFixture(t *testing.T) []*item.Item {
	t.Helper()

	path := writeItemFile(t, t.TempDir(), "tk-002.md",
		"---\nid: TK-002\ntitle: Abstraction\ntype: Task\nstatus: todo\ndependencies: [TK-001]\n---\n")

	return []*item.Item{
		{ID: "TK-001", Title: "Scaffolding", Status: item.StatusDone},
		{ID: "TK-002", Title: "Abstraction", Status: item.StatusTodo, Dependencies: []string{"TK-001"}, FilePath: path},
		{ID: "TK-003", Title: "Error handling", Status: item.StatusTodo},
		{ID: "TK-004", Title: "Logging", Status: item.StatusTodo},
		{ID: "ST-001", Title: "Command\nparsing", Status: item.StatusTodo, Dependencies: []string{"TK-002"}},
		{ID: "ST-002", Title: "Profile\nparsing", Status: item.StatusTodo,
			Dependencies: []string{"TK-002", "TK-003", "TK-004"}},
	}
}

func TestCompleteCmd_PinText(t *testing.T) {
	rc, buf := newTestContext(t, completeFixture(t), false)

	if err := (&CompleteCmd{ID: "TK-002"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "Completed TK-002: Abstraction\n" +
		"\nNow unblocked:\n" +
		"  ST-001  Command parsing\n" +
		"\nStill blocked (partial deps resolved):\n" +
		"  ST-002  Profile parsing (waiting on: TK-003, TK-004)\n"
	if got := buf.String(); got != want {
		t.Errorf("output:\n%q\nwant:\n%q", got, want)
	}
}

func TestCompleteCmd_PinTextNothingFreed(t *testing.T) {
	items := completeFixture(t)[:4]
	rc, buf := newTestContext(t, items, false)

	if err := (&CompleteCmd{ID: "TK-002"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got, want := buf.String(), "Completed TK-002: Abstraction\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestCompleteCmd_PinJSON(t *testing.T) {
	rc, buf := newTestContext(t, completeFixture(t), true)

	if err := (&CompleteCmd{ID: "TK-002"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	want := map[string]any{
		"id":    "TK-002",
		"title": "Abstraction",
		"unblocked": []any{
			map[string]any{"id": "ST-001", "title": "Command\nparsing"},
		},
		"still_blocked": []any{
			map[string]any{"id": "ST-002", "title": "Profile\nparsing", "waiting_on": []any{"TK-003", "TK-004"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON = %v, want %v", got, want)
	}
}

func TestCompleteCmd_PinJSONNothingFreed(t *testing.T) {
	rc, buf := newTestContext(t, completeFixture(t)[:4], true)

	if err := (&CompleteCmd{ID: "TK-002"}).Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := `{"id":"TK-002","title":"Abstraction","unblocked":[],"still_blocked":[]}`
	if got := strings.Join(strings.Fields(buf.String()), ""); got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestAddCmd_PinResolveIDErrors(t *testing.T) {
	tests := []struct {
		name    string
		skipped []string
		a       AddCmd
		wantErr string
	}{
		{
			name:    "--id a skipped file may hold",
			skipped: []string{"TK-100-old.md"},
			a:       AddCmd{Title: "New", ID: "TK-100"},
			wantErr: `item "TK-100" may already exist in <dir>/tasks/TK-100-old.md, which was skipped as unreadable; ` +
				"make it readable or pass another --id",
		},
		{
			name:    "--id of an existing item",
			a:       AddCmd{Title: "New", ID: "TK-002"},
			wantErr: `item "TK-002" already exists`,
		},
		{
			name:    "--prefix that cannot be part of a file name",
			a:       AddCmd{Title: "New", Prefix: "a/b-"},
			wantErr: `invalid prefix "a/b-": it becomes part of the file name`,
		},
		{
			name:    "a type with no numbered ids",
			a:       AddCmd{Title: "New", Type: "Bug"},
			wantErr: `cannot pick an id: no items of type "Bug" have a numbered id; pass --prefix (e.g. --prefix TK-) or --id`,
		},
		{
			name:    "no type and no untyped numbered ids",
			a:       AddCmd{Title: "New"},
			wantErr: "cannot pick an id: no untyped items have a numbered id; pass --prefix (e.g. --prefix TK-) or --id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := addTestDir(t)

			for _, name := range tt.skipped {
				if err := os.Symlink("nowhere.md", filepath.Join(dir, "tasks", name)); err != nil {
					t.Fatal(err)
				}
			}

			rc, _ := newAddContext(t, dir, defaultConfig(), false)
			tt.a.stdin = strings.NewReader("")

			err := tt.a.Run(rc)
			want := strings.ReplaceAll(tt.wantErr, "<dir>", dir)
			if err == nil || err.Error() != want {
				t.Errorf("error = %v, want %s", err, want)
			}
		})
	}
}
