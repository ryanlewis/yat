package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testItemID = "TK-001"

// TestMain builds the yat binary once before all e2e tests.
func TestMain(m *testing.M) {
	if err := exec.Command("go", "build", "-o", "yat_test_bin", ".").Run(); err != nil {
		panic("failed to build yat binary: " + err.Error())
	}
	abs, err := filepath.Abs("yat_test_bin")
	if err != nil {
		panic("failed to resolve yat binary path: " + err.Error())
	}
	yatBinPath = abs
	code := m.Run()
	os.Remove("yat_test_bin")
	os.Exit(code)
}

var yatBinPath string

func yatBin() string {
	return yatBinPath
}

// run executes yat with the given args and returns stdout, stderr, and exit code.
func run(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(yatBin(), args...)
	cmd.Dir = dir

	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run yat: %v", err)
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// writeItem creates a .md item file in the given directory.
func writeItem(t *testing.T, dir, filename, content string) {
	t.Helper()

	parent := filepath.Dir(filepath.Join(dir, filename))
	os.MkdirAll(parent, 0o755)

	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", filename, err)
	}
}

// setupProject creates a temp dir with a standard set of items.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "tk-001.md", `---
id: TK-001
title: "Set up CI"
type: Task
priority: Critical
points: 2
dependencies: []
status: draft
phase: "1"
---

## Acceptance Criteria
- CI runs on every push
`)

	writeItem(t, items, "tk-002.md", `---
id: TK-002
title: "Add linting"
type: Task
priority: High
points: 3
dependencies: [TK-001]
status: draft
phase: "1"
---

## Details
Configure golangci-lint.
`)

	writeItem(t, items, "st-001.md", `---
id: ST-001
title: "User auth"
type: Story
priority: Critical
points: 8
dependencies: [TK-001, TK-002]
status: draft
phase: "2"
---

Implement basic auth flow.
`)

	return dir
}

// --- Help and flags ---

func TestE2E_Help(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, _, code := run(t, dir, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"yat", "ready", "next", "show", "start", "complete", "blocked", "status", "graph"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q", want)
		}
	}
	if !strings.Contains(stdout, "yat: ignore") {
		t.Error("help should mention yat: ignore directive")
	}
}

func TestE2E_NoArgs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := run(t, dir)
	if code != 0 {
		t.Errorf("expected exit 0 for no subcommand, got %d", code)
	}
	if !strings.Contains(stderr, "yat --help") {
		t.Error("expected hint to run --help")
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, _, code := run(t, dir, "bogus")
	if code == 0 {
		t.Error("expected non-zero exit for unknown command")
	}
}

// --- Status command ---

func TestE2E_Status_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "3 items") {
		t.Errorf("expected '3 items', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "initial:") {
		t.Errorf("expected 'initial:' in output, got:\n%s", stdout)
	}
}

func TestE2E_Status_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "status", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["total_items"].(float64) != 3 {
		t.Errorf("total_items = %v, want 3", result["total_items"])
	}
}

// --- Ready command ---

func TestE2E_Ready_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "ready")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	// Only TK-001 has no deps
	if !strings.Contains(stdout, testItemID) {
		t.Errorf("expected TK-001 in ready, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "TK-002") || strings.Contains(stdout, "ST-001") {
		t.Errorf("blocked items should not appear in ready, got:\n%s", stdout)
	}
}

func TestE2E_Ready_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "ready", "-j")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result []map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 ready item, got %d", len(result))
	}
	if result[0]["id"] != testItemID {
		t.Errorf("id = %v, want TK-001", result[0]["id"])
	}
}

func TestE2E_Ready_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	stdout, _, code := run(t, dir, "ready")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "No items are ready") {
		t.Errorf("expected empty message, got:\n%s", stdout)
	}
}

// --- Next command ---

func TestE2E_Next_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "next")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, testItemID) {
		t.Errorf("expected TK-001, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Acceptance Criteria") {
		t.Errorf("expected body content, got:\n%s", stdout)
	}
}

func TestE2E_Next_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "next", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != testItemID {
		t.Errorf("id = %v, want TK-001", result["id"])
	}
}

// --- Show command ---

func TestE2E_Show_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "show", "TK-002")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "TK-002") || !strings.Contains(stdout, "Add linting") {
		t.Errorf("expected item details, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, testItemID) {
		t.Errorf("expected dependency TK-001 listed, got:\n%s", stdout)
	}
}

func TestE2E_Show_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "show", "ST-001", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != "ST-001" {
		t.Errorf("id = %v, want ST-001", result["id"])
	}
	deps := result["dependencies"].([]any)
	if len(deps) != 2 {
		t.Errorf("dependencies = %v, want 2 items", deps)
	}
}

func TestE2E_Show_NotFound(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	_, stderr, code := run(t, dir, "show", "GHOST")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown ID")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected ID in error, got: %s", stderr)
	}
}

// --- Blocked command ---

func TestE2E_Blocked_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "blocked")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "TK-002") {
		t.Errorf("expected TK-002 blocked, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ST-001") {
		t.Errorf("expected ST-001 blocked, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, testItemID) {
		t.Errorf("expected 'waiting on TK-001', got:\n%s", stdout)
	}
}

func TestE2E_Blocked_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "blocked", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result []map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 blocked, got %d", len(result))
	}
}

// --- Graph command ---

func TestE2E_Graph_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "graph")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "Layer 0:") {
		t.Errorf("expected 'Layer 0:', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "->") {
		t.Errorf("expected edges, got:\n%s", stdout)
	}
}

func TestE2E_Graph_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "graph", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	layers := result["layers"].([]any)
	if len(layers) < 2 {
		t.Errorf("expected at least 2 layers, got %d", len(layers))
	}
	edges := result["edges"].([]any)
	if len(edges) == 0 {
		t.Error("expected edges")
	}
}

// --- Start command ---

func TestE2E_Start_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "start", testItemID)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "Started TK-001") {
		t.Errorf("expected 'Started TK-001', got:\n%s", stdout)
	}

	// Verify file was updated
	data, err := os.ReadFile(filepath.Join(dir, "spec", "tk-001.md"))
	if err != nil {
		t.Fatalf("reading item file: %v", err)
	}
	if !strings.Contains(string(data), "status: in-progress") {
		t.Error("file not updated to in-progress")
	}
}

func TestE2E_Start_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "start", testItemID, "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != testItemID {
		t.Errorf("id = %v, want TK-001", result["id"])
	}
}

func TestE2E_Start_Blocked(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	_, stderr, code := run(t, dir, "start", "TK-002")
	if code == 0 {
		t.Fatal("expected non-zero exit for blocked item")
	}
	if !strings.Contains(stderr, "blocked") {
		t.Errorf("expected 'blocked' in error, got: %s", stderr)
	}
}

func TestE2E_Start_NotFound(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	_, stderr, code := run(t, dir, "start", "GHOST")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown ID")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected ID in error, got: %s", stderr)
	}
}

// --- Complete command ---

func TestE2E_Complete_Text(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "complete", testItemID)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "Completed TK-001") {
		t.Errorf("expected 'Completed TK-001', got:\n%s", stdout)
	}
	// TK-002 should now be unblocked (only dep was TK-001)
	if !strings.Contains(stdout, "TK-002") {
		t.Errorf("expected TK-002 in unblocked, got:\n%s", stdout)
	}

	// Verify file was updated
	data, err := os.ReadFile(filepath.Join(dir, "spec", "tk-001.md"))
	if err != nil {
		t.Fatalf("reading item file: %v", err)
	}
	if !strings.Contains(string(data), "status: done") {
		t.Error("file not updated to done")
	}
}

func TestE2E_Complete_JSON(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "complete", testItemID, "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != testItemID {
		t.Errorf("id = %v, want TK-001", result["id"])
	}
	unblocked := result["unblocked"].([]any)
	if len(unblocked) != 1 {
		t.Errorf("unblocked = %v, want 1 item", unblocked)
	}
}

func TestE2E_Complete_AlreadyDone(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	// Complete TK-001 first
	run(t, dir, "complete", testItemID)
	// Try again
	_, stderr, code := run(t, dir, "complete", testItemID)
	if code == 0 {
		t.Fatal("expected non-zero exit for already-done item")
	}
	if !strings.Contains(stderr, "already done") {
		t.Errorf("expected 'already done' in error, got: %s", stderr)
	}
}

func TestE2E_Complete_StillBlocked(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "complete", testItemID)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	// ST-001 depends on TK-001 (now done) and TK-002 (still draft) → still blocked
	if !strings.Contains(stdout, "Still blocked") {
		t.Errorf("expected 'Still blocked' section, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "ST-001") {
		t.Errorf("expected ST-001 in still-blocked, got:\n%s", stdout)
	}
}

// --- Workflow: start then complete ---

func TestE2E_Workflow_StartThenComplete(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	// Start TK-001
	stdout, _, code := run(t, dir, "start", testItemID)
	if code != 0 {
		t.Fatalf("start exit code = %d", code)
	}
	if !strings.Contains(stdout, "Started TK-001") {
		t.Error("expected 'Started TK-001'")
	}

	// Status should show in-progress
	stdout, _, _ = run(t, dir, "status", "--json")
	var status map[string]any
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	byStatus := status["by_status"].(map[string]any)
	active := byStatus["active"].(map[string]any)
	if active["count"].(float64) != 1 {
		t.Errorf("active count = %v, want 1", active["count"])
	}

	// Complete TK-001
	stdout, _, code = run(t, dir, "complete", testItemID)
	if code != 0 {
		t.Fatalf("complete exit code = %d", code)
	}
	if !strings.Contains(stdout, "Completed TK-001") {
		t.Error("expected 'Completed TK-001'")
	}

	// TK-002 should now be ready
	stdout, _, _ = run(t, dir, "ready", "--json")
	var ready []map[string]any
	if err := json.Unmarshal([]byte(stdout), &ready); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(ready) != 1 || ready[0]["id"] != "TK-002" {
		t.Errorf("expected TK-002 ready, got: %v", ready)
	}
}

// --- Full workflow: complete everything ---

func TestE2E_Workflow_CompleteAll(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)

	// Complete TK-001 → TK-002 unblocked
	run(t, dir, "complete", testItemID)
	// Complete TK-002 → ST-001 unblocked
	run(t, dir, "complete", "TK-002")
	// Complete ST-001
	run(t, dir, "complete", "ST-001")

	// Everything done, nothing ready or blocked
	stdout, _, _ := run(t, dir, "ready")
	if !strings.Contains(stdout, "No items are ready") {
		t.Errorf("expected no ready items, got:\n%s", stdout)
	}

	stdout, _, _ = run(t, dir, "blocked")
	if !strings.Contains(stdout, "No items are blocked") {
		t.Errorf("expected no blocked items, got:\n%s", stdout)
	}

	stdout, _, _ = run(t, dir, "status", "--json")
	var status map[string]any
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	byStatus2 := status["by_status"].(map[string]any)
	doneGroup := byStatus2["done"].(map[string]any)
	if doneGroup["count"].(float64) != 3 {
		t.Errorf("done count = %v, want 3", doneGroup["count"])
	}
}

// --- --dir flag ---

func TestE2E_DirFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom-items")
	os.MkdirAll(custom, 0o755)

	writeItem(t, custom, "item.md", "---\nid: X-001\ntitle: \"Custom dir item\"\nstatus: draft\n---\n")

	stdout, _, code := run(t, dir, "--dir", custom, "ready")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout, "X-001") {
		t.Errorf("expected X-001, got:\n%s", stdout)
	}
}

func TestE2E_DirFlag_Nonexistent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := run(t, dir, "--dir", "/nonexistent/path", "status")
	if code == 0 {
		t.Fatal("expected non-zero exit for nonexistent dir")
	}
	if !strings.Contains(stderr, "error") {
		t.Errorf("expected error in stderr, got: %s", stderr)
	}
}

func TestE2E_YAT_DIR_Env(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	custom := filepath.Join(dir, "env-items")
	os.MkdirAll(custom, 0o755)

	writeItem(t, custom, "item.md", "---\nid: ENV-001\ntitle: \"Env dir item\"\nstatus: draft\n---\n")

	cmd := exec.Command(yatBin(), "ready")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "YAT_DIR="+custom)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("yat failed: %v", err)
	}
	if !strings.Contains(string(out), "ENV-001") {
		t.Errorf("expected ENV-001, got:\n%s", string(out))
	}
}

// --- Error cases ---

func TestE2E_DuplicateIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "a.md", "---\nid: DUP\ntitle: \"First\"\nstatus: draft\n---\n")
	writeItem(t, items, "b.md", "---\nid: DUP\ntitle: \"Second\"\nstatus: draft\n---\n")

	_, stderr, code := run(t, dir, "status")
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate IDs")
	}
	if !strings.Contains(stderr, "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %s", stderr)
	}
}

func TestE2E_CyclicDeps(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "a.md", "---\nid: A\ntitle: \"A\"\nstatus: draft\ndependencies: [B]\n---\n")
	writeItem(t, items, "b.md", "---\nid: B\ntitle: \"B\"\nstatus: draft\ndependencies: [A]\n---\n")

	_, stderr, code := run(t, dir, "status")
	if code == 0 {
		t.Fatal("expected non-zero exit for cycle")
	}
	if !strings.Contains(stderr, "cycle") {
		t.Errorf("expected 'cycle' in error, got: %s", stderr)
	}
}

func TestE2E_InvalidStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "bad.md", "---\nid: BAD\ntitle: \"Bad\"\nstatus: banana\n---\n")

	_, stderr, code := run(t, dir, "status")
	if code == 0 {
		t.Fatal("expected non-zero exit for invalid status")
	}
	if !strings.Contains(stderr, "banana") {
		t.Errorf("expected 'banana' in error, got: %s", stderr)
	}
}

func TestE2E_UnknownDependency(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "a.md", "---\nid: A\ntitle: \"A\"\nstatus: draft\ndependencies: [GHOST]\n---\n")

	_, stderr, code := run(t, dir, "status")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown dep")
	}
	if !strings.Contains(stderr, "GHOST") {
		t.Errorf("expected 'GHOST' in error, got: %s", stderr)
	}
}

// --- Noise files are ignored ---

func TestE2E_NoiseFilesIgnored(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	items := filepath.Join(dir, "spec")

	// Add noise files
	writeItem(t, items, "notes.md", "# Just notes\nNo frontmatter.\n")
	writeItem(t, items, "design.md", "---\ntitle: \"Design doc\"\nauthor: \"Someone\"\n---\nNot an item.\n")
	writeItem(t, items, "ignored.md", "---\nid: SKIP-001\nstatus: draft\nyat: ignore\n---\n")
	writeItem(t, items, "data.txt", "id: SNEAKY\nstatus: draft\n")

	stdout, _, code := run(t, dir, "status", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if result["total_items"].(float64) != 3 {
		t.Errorf("total_items = %v, want 3 (noise should be ignored)", result["total_items"])
	}
}

// --- ErrMissingID warning goes to stderr ---

func TestE2E_MissingIDWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := filepath.Join(dir, "spec")
	os.MkdirAll(items, 0o755)

	writeItem(t, items, "valid.md", "---\nid: OK-001\nstatus: draft\n---\n")
	writeItem(t, items, "missing-id.md", "---\ntype: Task\nstatus: draft\n---\n")

	stdout, stderr, code := run(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr)
	}
	// Warning should go to stderr, not stdout
	if !strings.Contains(stderr, "warning") {
		t.Errorf("expected warning in stderr, got: %q", stderr)
	}
	// Stdout should still work
	if !strings.Contains(stdout, "1 items") {
		t.Errorf("expected '1 items' in stdout, got:\n%s", stdout)
	}
}

// --- Short flag -j ---

func TestE2E_ShortJSONFlag(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	stdout, _, code := run(t, dir, "status", "-j")
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON with -j flag: %v", err)
	}
}
