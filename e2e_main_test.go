package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ryanlewis/yat/internal/item"
)

// These tests pin the branches of main and newRunContext that the other e2e
// tests do not reach: init, agents and skill, a config load error, a read-only
// project, and failed reads and writes on stdin and stdout.

// runWith executes yat in dir with stdin, stdout and extra env set, and
// returns stderr and the exit code. env is appended to os.Environ() so the
// binary keeps GOCOVERDIR under `make test`.
func runWith(
	t *testing.T, dir string, stdin io.Reader, stdout io.Writer, env []string, args ...string,
) (stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(yatBin(), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout

	var errBuf strings.Builder
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("failed to run yat: %v", err)
		}
		exitCode = exitErr.ExitCode()
	}

	return errBuf.String(), exitCode
}

// skillEnv points every agent's skill directory under home.
func skillEnv(home string) []string {
	return []string{"HOME=" + home, "CLAUDE_CONFIG_DIR=", "CODEX_HOME=", "PI_CODING_AGENT_DIR="}
}

func TestE2E_Init(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var out strings.Builder
	stderr, code := runWith(t, dir, strings.NewReader("y\n"), &out, nil, "init", "items")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got: %s", stderr)
	}
	for _, want := range []string{"Proceed? [Y/n]: ", "Initialized yat project.", "items:   items/", "example: items/example.md"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("init output missing %q, got:\n%s", want, out.String())
		}
	}

	cfg, err := os.ReadFile(filepath.Join(dir, ".yat.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != "dir: items\n" {
		t.Errorf(".yat.yaml = %q, want %q", cfg, "dir: items\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "items", "example.md")); err != nil {
		t.Errorf("sample item not created: %v", err)
	}
}

func TestE2E_Init_AlreadyInitialized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeItem(t, dir, ".yat.yaml", "dir: spec\n")

	var out strings.Builder
	stderr, code := runWith(t, dir, strings.NewReader("y\n"), &out, nil, "init", "items")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if want := "error: yat is already initialized (.yat.yaml exists)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if out.String() != "" {
		t.Errorf("expected empty stdout, got: %s", out.String())
	}
}

func TestE2E_Agents_MatchesSkillShow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	agents, stderr, code := run(t, dir, "agents")
	if code != 0 || stderr != "" {
		t.Fatalf("agents: exit %d, stderr: %s", code, stderr)
	}
	if !strings.HasPrefix(agents, "---\nname: yat\n") {
		t.Errorf("agents should print the skill frontmatter first, got:\n%.200s", agents)
	}

	show, _, code := run(t, dir, "skill", "show")
	if code != 0 {
		t.Fatalf("skill show: exit %d", code)
	}
	if agents != show {
		t.Error("agents output differs from skill show")
	}
}

func TestE2E_SkillList_JSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	home := t.TempDir()

	var out strings.Builder
	stderr, code := runWith(t, dir, nil, &out, skillEnv(home), "skill", "list", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}

	var rows []struct {
		Agent     string `json:"agent"`
		Dir       string `json:"dir"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal([]byte(out.String()), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}

	want := map[string]string{
		"claude": filepath.Join(home, ".claude", "skills", "yat"),
		"codex":  filepath.Join(home, ".codex", "skills", "yat"),
		"pi":     filepath.Join(home, ".pi", "agent", "skills", "yat"),
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d agents, want %d: %+v", len(rows), len(want), rows)
	}
	for _, r := range rows {
		if r.Dir != want[r.Agent] {
			t.Errorf("%s dir = %q, want %q", r.Agent, r.Dir, want[r.Agent])
		}
		if r.Installed {
			t.Errorf("%s installed = true, want false", r.Agent)
		}
	}
}

func TestE2E_SkillShow_UnknownAgent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	stdout, stderr, code := run(t, dir, "skill", "show", "nosuch")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if want := "error: unknown agent \"nosuch\" (supported: claude, codex, pi)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %s", stdout)
	}
}

func TestE2E_MalformedConfig(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	writeItem(t, dir, ".yat.yaml", "dir: [\n")

	stdout, stderr, code := run(t, dir, "ready")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	wantPrefix := "error: parsing config " + filepath.Join(dir, ".yat.yaml") + ": yaml: "
	if !strings.HasPrefix(stderr, wantPrefix) {
		t.Errorf("stderr = %q, want prefix %q", stderr, wantPrefix)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %s", stdout)
	}
}

func TestE2E_ReadOnly_ReadCommand(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	writeItem(t, dir, ".yat.yaml", "readonly: true\n")

	stdout, stderr, code := run(t, dir, "ready", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, testItemID) {
		t.Errorf("expected %s in ready output, got:\n%s", testItemID, stdout)
	}
}

// A writer in a read-only project refuses without waiting for the items
// directory's lock, which another run may hold.
func TestE2E_ReadOnly_WriterSkipsLock(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	writeItem(t, dir, ".yat.yaml", "readonly: true\n")

	unlock, err := item.LockDir(filepath.Join(dir, "spec"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	start := time.Now()
	stdout, stderr, code := run(t, dir, "start", testItemID)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("start took %v, want it not to wait for the lock", elapsed)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if want := "error: project is read-only (readonly: true in .yat.yaml)\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %s", stdout)
	}
}

func TestE2E_Add_BodyReadError(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	// A regular file counts as piped input, and opened write-only it cannot be read.
	stdin, err := os.OpenFile(filepath.Join(t.TempDir(), "stdin"), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()

	var out strings.Builder
	stderr, code := runWith(t, dir, stdin, &out, nil, "add", "Unread body", "--type", "task")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: reading body from stdin: ") {
		t.Errorf("stderr = %q, want a body read error", stderr)
	}
	if out.String() != "" {
		t.Errorf("expected empty stdout, got: %s", out.String())
	}

	entries, err := os.ReadDir(filepath.Join(dir, "spec"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "unread") {
			t.Errorf("add created %s despite the read error", e.Name())
		}
	}
}

func TestE2E_NoArgs_NoProject_WriteError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Opened read-only, the file cannot take the onboarding hint.
	path := filepath.Join(t.TempDir(), "stdout")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()

	stderr, code := runWith(t, dir, nil, stdout, nil)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: write ") {
		t.Errorf("stderr = %q, want a write error", stderr)
	}
}
