package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runInit runs yat init in dir with input on stdin.
func runInit(t *testing.T, dir, input string, args ...string) (stdout, stderr string) {
	t.Helper()

	cmd := exec.Command(yatBin(), append([]string{"init"}, args...)...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		t.Fatalf("yat init: %v\nstderr:\n%s", err, errBuf.String())
	}

	return outBuf.String(), errBuf.String()
}

func TestE2E_Init_JSON_StdoutIsOnlyJSON(t *testing.T) {
	dir := t.TempDir()

	stdout, stderr := runInit(t, dir, "y\n", "items", "--json")

	dec := json.NewDecoder(strings.NewReader(stdout))

	var result map[string]any
	if err := dec.Decode(&result); err != nil {
		t.Fatalf("stdout is not JSON: %v\nstdout:\n%s", err, stdout)
	}

	if dec.More() {
		t.Errorf("stdout holds more than one JSON value:\n%s", stdout)
	}

	if result["dir"] != "items" {
		t.Errorf("dir = %v, want items", result["dir"])
	}

	if !strings.Contains(stderr, "Proceed? [Y/n]: ") {
		t.Errorf("expected the plan and prompt on stderr, got:\n%s", stderr)
	}

	if _, err := os.Stat(filepath.Join(dir, ".yat.yaml")); err != nil {
		t.Errorf(".yat.yaml not created: %v", err)
	}
}
