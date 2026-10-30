package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// mkdirs creates each relative path under root.
func mkdirs(t *testing.T, root string, paths ...string) {
	t.Helper()

	for _, p := range paths {
		if err := os.MkdirAll(filepath.Join(root, p), 0o755); err != nil {
			t.Fatalf("MkdirAll %s: %v", p, err)
		}
	}
}

// completions runs the completer on input with the cursor at the end and
// returns the candidates as strings.
func completions(t *testing.T, input string) (got []string, length int) {
	t.Helper()

	line := []rune(input)
	candidates, length := dirAutoCompleter{}.Do(line, len(line))

	got = make([]string, 0, len(candidates))
	for _, c := range candidates {
		got = append(got, string(c))
	}

	return got, length
}

func assertCompletions(t *testing.T, input string, want []string, wantLen int) {
	t.Helper()

	got, length := completions(t, input)
	if !slices.Equal(got, want) {
		t.Errorf("Do(%q) candidates = %q, want %q", input, got, want)
	}

	if length != wantLen {
		t.Errorf("Do(%q) length = %d, want %d", input, length, wantLen)
	}
}

// homeFixture sets HOME to a temp dir holding two directories, a hidden
// directory and a file.
func homeFixture(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	mkdirs(t, home, "docs", "projects", ".config")

	if err := os.WriteFile(filepath.Join(home, "notes.md"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return home
}

func TestDirAutoCompleter_BareTilde(t *testing.T) {
	homeFixture(t)

	// "~" alone completes to "~/<dir>/": the slash is prepended and nothing is replaced.
	assertCompletions(t, "~", []string{"/docs/", "/projects/"}, 0)
}

func TestDirAutoCompleter_BareTildeNoHome(t *testing.T) {
	t.Setenv("HOME", "")

	assertCompletions(t, "~", nil, 0)
}

func TestDirAutoCompleter_TildeSlash(t *testing.T) {
	homeFixture(t)

	assertCompletions(t, "~/", []string{"docs/", "projects/"}, 0)
}

func TestDirAutoCompleter_TildePrefix(t *testing.T) {
	homeFixture(t)

	assertCompletions(t, "~/pro", []string{"jects/"}, 3)
}

func TestDirAutoCompleter_TildeUserNotExpanded(t *testing.T) {
	dir := chdirTemp(t)
	homeFixture(t)
	mkdirs(t, dir, "~bob/work")

	// "~bob" is not the current user's home, so it is searched as a relative path.
	assertCompletions(t, "~bob/", []string{"work/"}, 0)
}

func TestDirAutoCompleter_AbsolutePath(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "alpha", "alpine", "beta")

	assertCompletions(t, root+"/al", []string{"pha/", "pine/"}, 2)
}

func TestDirAutoCompleter_RootDir(t *testing.T) {
	// A single leading slash searches "/", not "".
	tmp := t.TempDir()
	top := strings.SplitN(strings.TrimPrefix(tmp, "/"), "/", 2)[0]

	if top == "" || strings.HasPrefix(top, ".") {
		t.Skipf("temp dir %q has no usable top-level component", tmp)
	}

	got, length := completions(t, "/"+top[:1])
	if length != 1 {
		t.Errorf("length = %d, want 1", length)
	}

	want := top[1:] + "/"
	for _, c := range got {
		if c == want {
			return
		}
	}

	t.Errorf("Do(%q) = %q, want it to include %q", "/"+top[:1], got, want)
}

func TestDirAutoCompleter_RelativeEmpty(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "spec", "docs", ".git")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Files and hidden directories are left out; os.ReadDir sorts by name.
	assertCompletions(t, "", []string{"docs/", "spec/"}, 0)
}

func TestDirAutoCompleter_RelativeNested(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "work/items", "work/ideas", "work/.items")

	assertCompletions(t, "work/i", []string{"deas/", "tems/"}, 1)
}

func TestDirAutoCompleter_HiddenPrefix(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, ".hidden")

	// Hidden directories are skipped even when the prefix asks for them.
	assertCompletions(t, ".h", nil, 2)
}

func TestDirAutoCompleter_NoMatch(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "spec")

	assertCompletions(t, "zzz", nil, 3)
}

func TestDirAutoCompleter_MissingDir(t *testing.T) {
	chdirTemp(t)

	assertCompletions(t, "nope/x", nil, 1)
}

func TestDirAutoCompleter_LengthCountsRunes(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "éa")

	assertCompletions(t, "é", []string{"a/"}, 1)
}

func TestDirAutoCompleter_UsesCursorPosition(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "spec", "docs")

	// Text after the cursor is ignored.
	line := []rune("sxyz")
	candidates, length := dirAutoCompleter{}.Do(line, 1)

	if len(candidates) != 1 || string(candidates[0]) != "pec/" {
		t.Errorf("candidates = %q, want [pec/]", candidates)
	}

	if length != 1 {
		t.Errorf("length = %d, want 1", length)
	}
}

func TestDirSuffixes_Prepend(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "one", "two")

	got := dirSuffixes(root, "", ">")
	want := [][]rune{[]rune(">one/"), []rune(">two/")}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("dirSuffixes = %q, want %q", got, want)
	}
}

func TestDirSuffixes_UnreadableDir(t *testing.T) {
	if got := dirSuffixes(filepath.Join(t.TempDir(), "missing"), "", ""); got != nil {
		t.Errorf("dirSuffixes = %q, want nil", got)
	}
}

func TestDirSuffixes_Symlinks(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "real")

	if err := os.WriteFile(filepath.Join(root, "file.md"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	links := map[string]string{
		"linkdir":  "real",
		"linkfile": "file.md",
		"dangling": "missing",
		".hidden":  "real",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("Symlink: %v", err)
		}
	}

	// A symlink to a directory is offered; links to files, dangling links and
	// hidden links are not.
	got := dirSuffixes(root, "", "")
	want := [][]rune{[]rune("linkdir/"), []rune("real/")}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("dirSuffixes = %q, want %q", got, want)
	}
}

// fakeStdin points os.Stdin, which readline reads by default, at a pipe
// holding input, and sends readline's own output to a discarded file.
func fakeStdin(t *testing.T, input string) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}

	if _, err = w.WriteString(input); err != nil {
		t.Fatalf("write pipe: %v", err)
	}

	if err = w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}

	origIn, origOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = r, devNull

	t.Cleanup(func() {
		os.Stdin, os.Stdout = origIn, origOut
		r.Close()
		devNull.Close()
	})
}

func TestPromptDirInteractive_ExistingDir(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "work")
	fakeStdin(t, "  work/  \n")

	var buf bytes.Buffer

	got, err := promptDirInteractive(&buf, strings.NewReader(""))
	if err != nil {
		t.Fatalf("promptDirInteractive: %v", err)
	}

	if got != "work" {
		t.Errorf("dir = %q, want work (trimmed and cleaned)", got)
	}

	out := buf.String()
	if !strings.Contains(out, "Directories: work/") {
		t.Errorf("expected directory suggestions, got:\n%s", out)
	}

	if strings.Contains(out, "does not exist") {
		t.Errorf("unexpected creation prompt for existing dir:\n%s", out)
	}
}

func TestPromptDirInteractive_DefaultConfirmed(t *testing.T) {
	chdirTemp(t)
	fakeStdin(t, "\n")

	var buf bytes.Buffer

	got, err := promptDirInteractive(&buf, strings.NewReader("y\n"))
	if err != nil {
		t.Fatalf("promptDirInteractive: %v", err)
	}

	if got != defaultDir {
		t.Errorf("dir = %q, want %s", got, defaultDir)
	}

	if !strings.Contains(buf.String(), `Directory "spec" does not exist`) {
		t.Errorf("expected creation prompt, got:\n%s", buf.String())
	}
}

func TestPromptDirInteractive_Tilde(t *testing.T) {
	chdirTemp(t)
	home := homeFixture(t)
	fakeStdin(t, "~/docs\n")

	got, err := promptDirInteractive(&bytes.Buffer{}, strings.NewReader(""))
	if err != nil {
		t.Fatalf("promptDirInteractive: %v", err)
	}

	if want := filepath.Join(home, "docs"); got != want {
		t.Errorf("dir = %q, want %q", got, want)
	}
}

func TestPromptDirInteractive_DeclineThenRetry(t *testing.T) {
	dir := chdirTemp(t)
	mkdirs(t, dir, "items")
	fakeStdin(t, "backlog\nitems\n")

	var buf bytes.Buffer

	got, err := promptDirInteractive(&buf, strings.NewReader("n\n"))
	if err != nil {
		t.Fatalf("promptDirInteractive: %v", err)
	}

	if got != "items" {
		t.Errorf("dir = %q, want items", got)
	}

	if !strings.Contains(buf.String(), `"backlog" does not exist`) {
		t.Errorf("expected warning about backlog, got:\n%s", buf.String())
	}
}

func TestPromptDirInteractive_Interrupt(t *testing.T) {
	chdirTemp(t)
	fakeStdin(t, "\x03")

	got, err := promptDirInteractive(&bytes.Buffer{}, strings.NewReader(""))
	if err == nil || err.Error() != "interrupted" {
		t.Fatalf("err = %v, want interrupted", err)
	}

	if got != "" {
		t.Errorf("dir = %q, want empty", got)
	}
}

func TestPromptDirInteractive_EOF(t *testing.T) {
	chdirTemp(t)
	fakeStdin(t, "")

	_, err := promptDirInteractive(&bytes.Buffer{}, strings.NewReader(""))
	if err == nil || err.Error() != "reading input: EOF" {
		t.Fatalf("err = %v, want reading input: EOF", err)
	}
}
