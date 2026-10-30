package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// runInProcess calls execute with empty stdin and returns stdout, stderr and the exit code.
func runInProcess(args ...string) (stdout, stderr string, code int) {
	var out, errOut strings.Builder
	code = execute(args, strings.NewReader(""), &out, &errOut)

	return out.String(), errOut.String(), code
}

func TestExecute_Help(t *testing.T) {
	stdout, stderr, code := runInProcess("--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Usage: yat") {
		t.Errorf("help missing usage line, got:\n%s", stdout)
	}
}

func TestExecute_UsageError(t *testing.T) {
	_, stderr, code := runInProcess("bogus")
	if code != 80 {
		t.Errorf("exit code = %d, want kong's usage error code 80", code)
	}
	if !strings.HasPrefix(stderr, "yat: error: ") {
		t.Errorf("stderr = %q, want a kong usage error", stderr)
	}
}

func TestExecute_SkillShow(t *testing.T) {
	stdout, stderr, code := runInProcess("skill", "show")
	if code != 0 || stderr != "" {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "---\nname: yat\n") {
		t.Errorf("skill show should print the skill, got:\n%.200s", stdout)
	}
}

func TestExecute_SkillShow_UnknownAgent(t *testing.T) {
	stdout, stderr, code := runInProcess("skill", "show", "nosuch")
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

// exitAfterParseEnv makes TestParse_ExitAfterParse run as the child process
// that calls kong's Exit after parsing.
const exitAfterParseEnv = "YAT_TEST_EXIT_AFTER_PARSE"

// A command that calls ctx.Exit, FatalIfErrorf or Fatalf after parse
// returns must exit with kong's code, not panic with the kongExit sentinel
// that only parse recovers.
func TestParse_ExitAfterParse(t *testing.T) {
	if os.Getenv(exitAfterParseEnv) == "1" {
		var c cli
		ctx, _, ok := parse(&c, []string{"skill", "show"}, io.Discard, io.Discard)
		if ok {
			ctx.Exit(3)
		}
		os.Exit(0)
	}

	child := exec.Command(os.Args[0], "-test.run=^TestParse_ExitAfterParse$")
	child.Env = append(os.Environ(), exitAfterParseEnv+"=1")
	out, err := child.CombinedOutput()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
		t.Errorf("child err = %v, want exit code 3", err)
	}
	if strings.Contains(string(out), "panic") {
		t.Errorf("child panicked:\n%s", out)
	}
}

func TestItemsDir(t *testing.T) {
	tests := []struct{ flag, config, want string }{
		{"flag", "config", "flag"},
		{"", "config", "config"},
		{"", "", "spec"},
	}
	for _, tt := range tests {
		if got := itemsDir(tt.flag, tt.config); got != tt.want {
			t.Errorf("itemsDir(%q, %q) = %q, want %q", tt.flag, tt.config, got, tt.want)
		}
	}
}
