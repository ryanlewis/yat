package main

import (
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
