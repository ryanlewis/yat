package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/config"
	"github.com/ryanlewis/yat/internal/item"
)

// newAddContext loads dir with cfg and captures output.
func newAddContext(t *testing.T, dir string, cfg *config.Config, jsonMode bool) (*RunContext, *bytes.Buffer) {
	t.Helper()

	rc, err := NewRunContext(dir, jsonMode, cfg)
	if err != nil {
		t.Fatalf("NewRunContext: %v", err)
	}

	var buf bytes.Buffer
	rc.Stdout = &buf

	return rc, &buf
}

// addTestDir holds tasks in tasks/ (TK-, three digits) and a story at the root.
func addTestDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "tasks"), 0o750); err != nil {
		t.Fatal(err)
	}

	writeItemFile(t, dir, "tasks/TK-002.md", "---\nid: TK-002\ntype: Task\nstatus: todo\n---\n")
	writeItemFile(t, dir, "tasks/TK-006.md", "---\nid: TK-006\ntype: Task\nstatus: done\n---\n")
	writeItemFile(t, dir, "ST-01.md", "---\nid: ST-01\ntype: Story\nstatus: todo\n---\n")

	return dir
}

func runAdd(t *testing.T, rc *RunContext, a *AddCmd) {
	t.Helper()

	if a.stdin == nil {
		a.stdin = strings.NewReader("")
	}

	if err := a.Run(rc); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestAddCmd_NextIDAndFolderForType(t *testing.T) {
	dir := addTestDir(t)
	rc, buf := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "Write the docs", Type: "Task", Priority: "high", Dep: []string{"TK-002"}, Points: 2})

	path := filepath.Join(dir, "tasks", "TK-007-write-the-docs.md")
	want := "---\nid: TK-007\ntitle: Write the docs\ntype: Task\npriority: High\npoints: 2\n" +
		"dependencies: [TK-002]\nstatus: todo\n---\n"

	if got := readFile(t, path); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}

	if got := buf.String(); got != "Added TK-007 at "+path+"\n" {
		t.Errorf("output = %q", got)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}

	// The new item loads with the rest.
	rc2, _ := newAddContext(t, dir, defaultConfig(), false)
	if it, ok := rc2.Graph.Item("TK-007"); !ok || it.Title != "Write the docs" {
		t.Errorf("TK-007 did not load back: %+v", it)
	}
}

func TestAddCmd_KeepsPaddingWidth(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "Second story", Type: "story"})

	if _, err := os.Stat(filepath.Join(dir, "ST-02-second-story.md")); err != nil {
		t.Errorf("expected ST-02 at the items root: %v", err)
	}
}

func TestAddCmd_Prefix(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "New thing", Type: "Bug", Prefix: "BG-"})

	if _, err := os.Stat(filepath.Join(dir, "BG-001-new-thing.md")); err != nil {
		t.Errorf("expected BG-001 at the items root: %v", err)
	}
}

func TestAddCmd_NoPrefixToInfer(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	err := (&AddCmd{Title: "x", Type: "Bug", stdin: strings.NewReader("")}).Run(rc)
	if err == nil || !strings.Contains(err.Error(), "--prefix") {
		t.Errorf("expected a hint to pass --prefix, got: %v", err)
	}
}

func TestAddCmd_SkippedFiles(t *testing.T) {
	tests := []struct {
		name    string
		skipped []string // broken symlinks in tasks/
		notMap  bool     // write tasks/README.md with scalar frontmatter
		a       AddCmd
		wantID  string
		wantErr string
	}{
		{
			name:    "a skipped file names a higher id of the prefix",
			skipped: []string{"TK-009-fix-login.md"},
			a:       AddCmd{Title: "New", Type: "Task"},
			wantID:  "TK-010",
		},
		{
			name:    "a skipped file names no id of the prefix",
			skipped: []string{"TK-009-fix-login.md", "notes.md"},
			a:       AddCmd{Title: "New", Type: "Task"},
			wantErr: "cannot pick an id: skipped as unreadable and may hold the next TK- id: <dir>/tasks/notes.md; " +
				"make them readable or pass --id",
		},
		{
			name:    "a skipped file names an id of another prefix",
			skipped: []string{"BUG-009.md"},
			a:       AddCmd{Title: "New", Prefix: "TK-"},
			wantErr: "may hold the next TK- id: <dir>/tasks/BUG-009.md;",
		},
		{
			name:   "a readable file whose frontmatter is not a mapping does not block",
			notMap: true,
			a:      AddCmd{Title: "New", Type: "Task"},
			wantID: "TK-007",
		},
		{
			name:    "a skipped file numbered wider than the ids is not counted",
			skipped: []string{"TK-20240115-retro.md"},
			a:       AddCmd{Title: "New", Type: "Task"},
			wantErr: "may hold the next TK- id: <dir>/tasks/TK-20240115-retro.md;",
		},
		{
			name:    "with no ids of the prefix, four digits count",
			skipped: []string{"BG-0042-crash.md"},
			a:       AddCmd{Title: "New", Prefix: "BG-"},
			wantID:  "BG-0043",
		},
		{
			name:    "--id refuses an id a skipped file names",
			skipped: []string{"TK-100-old.md"},
			a:       AddCmd{Title: "New", Type: "Task", ID: "TK-100"},
			wantErr: `item "TK-100" may already exist in <dir>/tasks/TK-100-old.md, which was skipped as unreadable`,
		},
		{
			name:    "--id refuses an id a skipped file is named",
			skipped: []string{"TK-100.md"},
			a:       AddCmd{Title: "New", Type: "Task", ID: "TK-100"},
			wantErr: `may already exist in <dir>/tasks/TK-100.md,`,
		},
		{
			name:    "--id allows a longer id that starts the same",
			skipped: []string{"TK-1000.md"},
			a:       AddCmd{Title: "New", Type: "Task", ID: "TK-100"},
			wantID:  "TK-100",
		},
		{
			name:    "--id still works",
			skipped: []string{"notes.md"},
			a:       AddCmd{Title: "New", Type: "Task", ID: "TK-100"},
			wantID:  "TK-100",
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

			if tt.notMap {
				writeItemFile(t, dir, "tasks/README.md", "---\nSome intro\n---\n")
			}

			rc, buf := newAddContext(t, dir, defaultConfig(), true)
			tt.a.stdin = strings.NewReader("")

			err := tt.a.Run(rc)
			if tt.wantErr != "" {
				want := strings.ReplaceAll(tt.wantErr, "<dir>", dir)
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v, want it to contain %q", err, want)
				}

				return
			}

			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if !strings.Contains(buf.String(), `"id": "`+tt.wantID+`"`) {
				t.Errorf("output = %s, want id %s", buf, tt.wantID)
			}
		})
	}
}

func TestAddCmd_ExplicitID(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	err := (&AddCmd{Title: "x", ID: "TK-006", stdin: strings.NewReader("")}).Run(rc)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected an existing id to be refused, got: %v", err)
	}

	err = (&AddCmd{Title: "x", ID: "../x", stdin: strings.NewReader("")}).Run(rc)
	if err == nil || !strings.Contains(err.Error(), "invalid id") {
		t.Errorf("expected a path in the id to be refused, got: %v", err)
	}

	runAdd(t, rc, &AddCmd{Title: "Custom", ID: "MY-ID"})

	if _, err := os.Stat(filepath.Join(dir, "MY-ID-custom.md")); err != nil {
		t.Errorf("expected MY-ID-custom.md: %v", err)
	}
}

func TestAddCmd_RefusesOverwrite(t *testing.T) {
	dir := addTestDir(t)
	writeItemFile(t, dir, "tasks/TK-007-dup.md", "not an item\n")
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	err := (&AddCmd{Title: "Dup", Type: "Task", stdin: strings.NewReader("")}).Run(rc)
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected refusal, got: %v", err)
	}

	if got := readFile(t, filepath.Join(dir, "tasks", "TK-007-dup.md")); got != "not an item\n" {
		t.Errorf("existing file changed: %q", got)
	}

	entries, _ := os.ReadDir(filepath.Join(dir, "tasks"))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestAddCmd_ReadOnly(t *testing.T) {
	dir := addTestDir(t)
	cfg := defaultConfig()
	cfg.ReadOnly = true
	rc, _ := newAddContext(t, dir, cfg, false)

	err := (&AddCmd{Title: "x", Type: "Task"}).Run(rc)
	if err == nil || err.Error() != "project is read-only (readonly: true in .yat.yaml)" {
		t.Errorf("expected read-only error, got: %v", err)
	}
}

func TestAddCmd_Validation(t *testing.T) {
	for name, a := range map[string]*AddCmd{
		"empty title":      {Title: "  ", Type: "Task"},
		"bad priority":     {Title: "x", Type: "Task", Priority: "urgent"},
		"bad status":       {Title: "x", Type: "Task", Status: "bogus"},
		"unknown dep":      {Title: "x", Type: "Task", Dep: []string{"NOPE"}},
		"negative points":  {Title: "x", Type: "Task", Points: -1},
		"prefix with path": {Title: "x", Prefix: "a/b-"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := addTestDir(t)
			rc, _ := newAddContext(t, dir, defaultConfig(), false)
			a.stdin = strings.NewReader("")

			if err := a.Run(rc); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAddCmd_BodyFromStdin(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "Piped", Type: "Task", stdin: strings.NewReader("Do the thing")})

	got := readFile(t, filepath.Join(dir, "tasks", "TK-007-piped.md"))
	if !strings.HasSuffix(got, "---\n\nDo the thing\n") {
		t.Errorf("body not written from stdin:\n%s", got)
	}

	runAdd(t, rc, &AddCmd{Title: "Flag", ID: "F-1", Body: ptr("From flag"), stdin: strings.NewReader("ignored")})

	if got := readFile(t, filepath.Join(dir, "F-1-flag.md")); !strings.HasSuffix(got, "\nFrom flag\n") {
		t.Errorf("--body not used:\n%s", got)
	}
}

func TestAddCmd_JSON(t *testing.T) {
	dir := addTestDir(t)
	rc, buf := newAddContext(t, dir, defaultConfig(), true)

	runAdd(t, rc, &AddCmd{Title: "Json", Type: "Task", Phase: "1", Body: ptr("b")})

	var got addJSON
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}

	want := addJSON{
		ID: "TK-007", Path: filepath.Join(dir, "tasks", "TK-007-json.md"), Title: "Json", Type: "Task",
		Dependencies: []string{}, Status: "todo", Phase: "1", Body: "b\n",
	}
	if got.ID != want.ID || got.Path != want.Path || got.Status != want.Status || got.Phase != want.Phase ||
		got.Body != want.Body || got.Dependencies == nil {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestAddCmd_FieldAliasesAndCustomStatus(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "TK-001.md", "---\nkey: TK-001\ntype: Task\nstate: backlog\n---\n")

	cfg := &config.Config{
		Statuses:     customStatuses(),
		FieldAliases: map[string]string{"id": "key", "status": "state"},
	}
	rc, _ := newAddContext(t, dir, cfg, false)

	runAdd(t, rc, &AddCmd{Title: "Aliased", Type: "Task"})

	want := "---\nkey: TK-002\ntitle: Aliased\ntype: Task\nstate: backlog\n---\n"
	if got := readFile(t, filepath.Join(dir, "TK-002-aliased.md")); got != want {
		t.Errorf("file =\n%s\nwant\n%s", got, want)
	}
}

func TestAddCmd_AliasCollision(t *testing.T) {
	dir := addTestDir(t)
	cfg := defaultConfig()
	cfg.FieldAliases = map[string]string{"phase": "status"}
	rc, _ := newAddContext(t, dir, cfg, false)

	err := (&AddCmd{Title: "x", Type: "Task", Phase: "1", stdin: strings.NewReader("")}).Run(rc)
	if err == nil || !strings.Contains(err.Error(), "field_aliases") {
		t.Errorf("expected a field_aliases error, got: %v", err)
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Fix: the  login bug!":    "fix-the-login-bug",
		"Über naïve café":         "über-naïve-café",
		"タスク 2":                   "タスク-2",
		"---":                     "",
		strings.Repeat("ab ", 30): strings.Repeat("ab-", 16) + "ab",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddCmd_PrefixEndingInDigit(t *testing.T) {
	dir := t.TempDir()
	writeItemFile(t, dir, "V2001-a.md", "---\nid: V2001\nstatus: todo\n---\n")
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "b", Prefix: "V2"})

	if _, err := os.Stat(filepath.Join(dir, "V2002-b.md")); err != nil {
		t.Errorf("expected V2002 after V2001: %v", err)
	}
}

func ptr(s string) *string { return &s }

func TestAddCmd_EmptyBodyFlagSkipsStdin(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "Empty", ID: "E-1", Body: ptr(""), stdin: strings.NewReader("not the body")})

	if got := readFile(t, filepath.Join(dir, "E-1-empty.md")); strings.Contains(got, "not the body") {
		t.Errorf("stdin read despite --body \"\":\n%s", got)
	}
}

func TestAddCmd_TypeTakesExistingCasing(t *testing.T) {
	dir := addTestDir(t)
	rc, _ := newAddContext(t, dir, defaultConfig(), false)

	runAdd(t, rc, &AddCmd{Title: "Lower", Type: "task"})

	got := readFile(t, filepath.Join(dir, "tasks", "TK-007-lower.md"))
	if !strings.Contains(got, "\ntype: Task\n") {
		t.Errorf("want type: Task, got:\n%s", got)
	}
}

func TestAddCmd_ConcurrentAddsTakeDistinctIDs(t *testing.T) {
	dir := addTestDir(t)

	// Every add runs at once, as when several yat add processes start together.
	const n = 8

	errs := make(chan error, n)

	for i := range n {
		go func() {
			errs <- runLocked(dir, &AddCmd{Title: fmt.Sprintf("Task %d", i), Type: "Task", stdin: strings.NewReader("")})
		}()
	}

	for range n {
		if err := <-errs; err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	items, _, err := item.LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}

	ids := map[string]bool{}
	for _, it := range items {
		if ids[it.ID] {
			t.Errorf("id %s taken twice", it.ID)
		}

		ids[it.ID] = true
	}

	if len(items) != 3+n {
		t.Errorf("got %d items, want %d", len(items), 3+n)
	}
}
