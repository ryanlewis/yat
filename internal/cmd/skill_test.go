package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryanlewis/yat/internal/skill"
)

const (
	testAgentClaude = "claude"
	testAgentCodex  = "codex"

	// oldSkill stands in for an earlier yat install: same header, old body.
	oldSkill = "---\nname: yat\n---\nold body\n"
)

func newSkillEnv(jsonMode, interactive bool, stdin string) (env *SkillEnv, stdout, stderr *bytes.Buffer) {
	stdout, stderr = &bytes.Buffer{}, &bytes.Buffer{}
	env = &SkillEnv{
		JSON:        jsonMode,
		Interactive: interactive,
		Stdout:      stdout,
		Stderr:      stderr,
		Stdin:       strings.NewReader(stdin),
	}

	return env, stdout, stderr
}

func skillFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading SKILL.md: %v", err)
	}

	return string(data)
}

func TestSkillInstall_FreshDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills", "yat")
	env, stdout, _ := newSkillEnv(false, false, "")

	if err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := skillFile(t, dir); got != skill.Render() {
		t.Error("installed SKILL.md does not match the rendered skill")
	}
	if !strings.Contains(stdout.String(), "Installed claude skill to "+dir) {
		t.Errorf("unexpected output: %s", stdout)
	}
}

func TestSkillInstall_ExistingNonInteractiveRefuses(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, _, _ := newSkillEnv(false, false, "")

	err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "-y") {
		t.Fatalf("expected error mentioning -y, got %v", err)
	}
	if got := skillFile(t, dir); got != oldSkill {
		t.Error("existing install was overwritten without confirmation")
	}
}

func TestSkillInstall_ExistingPromptDeclined(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, stderr := newSkillEnv(false, true, "\n")

	if err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr.String(), "Overwrite?") {
		t.Errorf("expected overwrite prompt, got: %s", stderr)
	}
	if !strings.Contains(stdout.String(), "Aborted.") {
		t.Errorf("expected Aborted., got: %s", stdout)
	}
	if got := skillFile(t, dir); got != oldSkill {
		t.Error("declined prompt still overwrote the install")
	}
}

func TestSkillInstall_ExistingPromptAccepted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, _, _ := newSkillEnv(false, true, "y\n")

	if err := (&SkillInstallCmd{Agent: testAgentCodex, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := skillFile(t, dir); got != skill.Render() {
		t.Error("accepted prompt did not overwrite the install")
	}
}

func TestSkillInstall_JSONNeverPrompts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, _, stderr := newSkillEnv(true, true, "y\n")

	if err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err == nil {
		t.Fatal("expected --json without -y to refuse an overwrite")
	}
	if stderr.Len() != 0 {
		t.Errorf("--json should not prompt, got: %s", stderr)
	}
}

func TestSkillInstall_YesJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, _ := newSkillEnv(true, false, "")

	if err := (&SkillInstallCmd{Agent: "pi", Path: dir, Yes: true}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got skillInstallJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if got.Agent != "pi" || got.Dir != dir || !got.Overwritten || len(got.Files) != 1 || got.Files[0] != "SKILL.md" {
		t.Errorf("unexpected JSON: %+v", got)
	}
}

func TestSkillInstall_UnknownAgent(t *testing.T) {
	env, _, _ := newSkillEnv(false, false, "")
	err := (&SkillInstallCmd{Agent: "bogus", Path: t.TempDir()}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "supported") {
		t.Fatalf("expected unknown agent error, got %v", err)
	}
}

func TestSkillUninstall_NonInteractiveRefuses(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, _, _ := newSkillEnv(false, false, "")

	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err == nil {
		t.Fatal("expected refusal without -y")
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Error("file removed without confirmation")
	}
}

func TestSkillUninstall_PromptListsFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "yat")
	os.Mkdir(dir, 0o750)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, stderr := newSkillEnv(false, true, "yes\n")

	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr.String(), "- SKILL.md") {
		t.Errorf("prompt should list files, got: %s", stderr)
	}
	if !strings.Contains(stdout.String(), "Removed claude skill") {
		t.Errorf("unexpected output: %s", stdout)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("--path dir should be kept, stat err = %v", err)
	}
}

func TestSkillUninstall_KeepsUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	os.WriteFile(filepath.Join(dir, "notes.md"), []byte("keep"), 0o600)
	env, stdout, _ := newSkillEnv(true, false, "")

	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir, Yes: true}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got skillUninstallJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if len(got.Removed) != 1 || got.Removed[0] != "SKILL.md" {
		t.Errorf("Removed = %v, want [SKILL.md]", got.Removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.md")); err != nil {
		t.Errorf("unrelated file removed: %v", err)
	}
}

// TestSkillUninstall_KeepsPathDir is the repro: a --path dir holding only
// yat's SKILL.md must survive uninstall.
func TestSkillUninstall_KeepsPathDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, _, _ := newSkillEnv(false, false, "")

	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir, Yes: true}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("SKILL.md not removed: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("--path dir removed: %v", err)
	}
}

// TestSkillUninstall_RemovesEmptyDefaultDir checks that the agent's own
// skills/yat dir still goes when uninstall empties it.
func TestSkillUninstall_RemovesEmptyDefaultDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", base)
	env, _, _ := newSkillEnv(false, false, "")

	if err := (&SkillInstallCmd{Agent: testAgentClaude}).Run(env); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Yes: true}).Run(env); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "skills", "yat")); !os.IsNotExist(err) {
		t.Errorf("default skill dir not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "skills")); err != nil {
		t.Errorf("parent skills dir removed: %v", err)
	}
}

func TestSkillUninstall_NothingInstalled(t *testing.T) {
	env, _, _ := newSkillEnv(false, false, "")
	err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: t.TempDir(), Yes: true}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "no claude skill installed") {
		t.Fatalf("expected not-installed error, got %v", err)
	}
}

func TestSkillShow(t *testing.T) {
	env, stdout, _ := newSkillEnv(false, false, "")
	if err := (&SkillShowCmd{}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() != skill.Render() {
		t.Error("skill show should print the rendered SKILL.md")
	}
}

func TestSkillShow_AgentJSON(t *testing.T) {
	env, stdout, _ := newSkillEnv(true, false, "")
	if err := (&SkillShowCmd{Agent: testAgentCodex}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got skillShowAgentJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if got.Agent != testAgentCodex || got.Files["SKILL.md"] != skill.Render() {
		t.Errorf("unexpected JSON: %+v", got)
	}
}

func TestAgentsMatchesSkillShow(t *testing.T) {
	for _, jsonMode := range []bool{false, true} {
		agentsEnv, agentsOut, _ := newSkillEnv(jsonMode, false, "")
		showEnv, showOut, _ := newSkillEnv(jsonMode, false, "")

		if err := (&AgentsCmd{}).Run(agentsEnv); err != nil {
			t.Fatalf("agents: %v", err)
		}
		if err := (&SkillShowCmd{}).Run(showEnv); err != nil {
			t.Fatalf("skill show: %v", err)
		}
		if agentsOut.String() != showOut.String() {
			t.Errorf("json=%v: yat agents output differs from yat skill show", jsonMode)
		}
	}
}

func TestSkillList_JSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")

	claudeDir := filepath.Join(home, ".claude", "skills", "yat")
	os.MkdirAll(claudeDir, 0o750)
	os.WriteFile(filepath.Join(claudeDir, "SKILL.md"), []byte(oldSkill), 0o600)

	env, stdout, _ := newSkillEnv(true, false, "")
	if err := (&SkillListCmd{}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var rows []skillListJSON
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	for _, r := range rows {
		want := r.Agent == testAgentClaude
		if r.Installed != want {
			t.Errorf("%s installed = %v, want %v", r.Agent, r.Installed, want)
		}
	}
	if rows[0].Dir != claudeDir {
		t.Errorf("claude dir = %q, want %q", rows[0].Dir, claudeDir)
	}
}

func TestSkillInstall_ForeignFileRefusedEvenWithYes(t *testing.T) {
	dir := t.TempDir()
	foreign := "---\nname: other\n---\n"
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(foreign), 0o600)
	env, _, _ := newSkillEnv(false, false, "")

	err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir, Yes: true}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "not written by yat") {
		t.Fatalf("expected refusal, got %v", err)
	}
	if got := skillFile(t, dir); got != foreign {
		t.Error("foreign SKILL.md was overwritten")
	}
}

// clearSkillHome unsets $HOME and every agent config variable, so no agent
// default dir resolves into the real home directory.
func clearSkillHome(t *testing.T) {
	t.Helper()
	for _, env := range []string{"HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "PI_CODING_AGENT_DIR"} {
		t.Setenv(env, "")
	}
}

// setupMixedSkillList leaves claude installed, codex not installed and pi
// unresolvable, so one list run reaches every row kind.
func setupMixedSkillList(t *testing.T) (claudeDir, codexDir string) {
	t.Helper()
	clearSkillHome(t)
	claudeBase, codexBase := t.TempDir(), t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeBase)
	t.Setenv("CODEX_HOME", codexBase)

	claudeDir = filepath.Join(claudeBase, "skills", "yat")
	os.MkdirAll(claudeDir, 0o750)
	os.WriteFile(filepath.Join(claudeDir, "SKILL.md"), []byte(oldSkill), 0o600)

	return claudeDir, filepath.Join(codexBase, "skills", "yat")
}

func TestSkillList_Text(t *testing.T) {
	claudeDir, codexDir := setupMixedSkillList(t)
	env, stdout, _ := newSkillEnv(false, false, "")

	if err := (&SkillListCmd{}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := "claude   " + claudeDir + "  (installed)\n" +
		"codex    " + codexDir + "  (not installed)\n" +
		"pi       (path unresolved: cannot locate the skill directory: " +
		"neither $PI_CODING_AGENT_DIR nor $HOME is set)\n" +
		"\nInstall with: yat skill install <agent>  (agents: claude, codex, pi)\n"
	if got := stdout.String(); got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestSkillList_JSONUnresolvedDir(t *testing.T) {
	claudeDir, _ := setupMixedSkillList(t)
	env, stdout, _ := newSkillEnv(true, false, "")

	if err := (&SkillListCmd{}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0]["installed"] != true || rows[0]["dir"] != claudeDir {
		t.Errorf("unexpected claude row: %v", rows[0])
	}
	pi := rows[2]
	if _, ok := pi["dir"]; ok {
		t.Errorf("unresolved row should omit dir: %v", pi)
	}
	if pi["agent"] != "pi" || pi["installed"] != false ||
		!strings.Contains(fmt.Sprint(pi["error"]), "$PI_CODING_AGENT_DIR") {
		t.Errorf("unexpected pi row: %v", pi)
	}
	if _, ok := rows[1]["error"]; ok {
		t.Errorf("resolved row should omit error: %v", rows[1])
	}
}

func TestSkillInstall_DefaultDirUnresolved(t *testing.T) {
	clearSkillHome(t)
	env, _, _ := newSkillEnv(false, false, "")

	err := (&SkillInstallCmd{Agent: testAgentCodex}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "neither $CODEX_HOME nor $HOME is set") {
		t.Fatalf("expected unresolved dir error, got %v", err)
	}
}

func TestSkillInstall_PromptEOFDeclines(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, _ := newSkillEnv(false, true, "")

	if err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stdout.String() != "Aborted.\n" {
		t.Errorf("unexpected output: %q", stdout)
	}
	if got := skillFile(t, dir); got != oldSkill {
		t.Error("closed stdin still overwrote the install")
	}
}

func TestSkillInstall_WriteError(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "SKILL.md"), 0o750)
	env, stdout, _ := newSkillEnv(true, false, "")

	if err := (&SkillInstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err == nil {
		t.Fatal("expected an error writing over a directory")
	}
	if stdout.Len() != 0 {
		t.Errorf("failed install should print nothing, got: %s", stdout)
	}
}

func TestSkillUninstall_UnknownAgent(t *testing.T) {
	env, _, _ := newSkillEnv(false, false, "")
	err := (&SkillUninstallCmd{Agent: "bogus", Path: t.TempDir(), Yes: true}).Run(env)
	if err == nil || !strings.Contains(err.Error(), `unknown agent "bogus"`) {
		t.Fatalf("expected unknown agent error, got %v", err)
	}
}

func TestSkillUninstall_PromptDeclined(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, stderr := newSkillEnv(false, true, "n\n")

	if err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr.String(), "Remove claude skill at "+dir+"? [y/N]: ") {
		t.Errorf("expected removal prompt, got: %s", stderr)
	}
	if stdout.String() != "Aborted.\n" {
		t.Errorf("unexpected output: %q", stdout)
	}
	if got := skillFile(t, dir); got != oldSkill {
		t.Error("declined prompt still removed the install")
	}
}

func TestSkillUninstall_JSONNeverPrompts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	env, stdout, stderr := newSkillEnv(true, true, "y\n")

	err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir}).Run(env)
	if err == nil || !strings.Contains(err.Error(), "pass -y to confirm") {
		t.Fatalf("expected refusal, got %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("--json should not prompt, stdout %q stderr %q", stdout, stderr)
	}
	if got := skillFile(t, dir); got != oldSkill {
		t.Error("refused uninstall changed the install")
	}
}

func TestSkillUninstall_RemoveError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(oldSkill), 0o600)
	os.Chmod(dir, 0o500)
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	env, stdout, _ := newSkillEnv(true, false, "")

	err := (&SkillUninstallCmd{Agent: testAgentClaude, Path: dir, Yes: true}).Run(env)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("expected permission error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("failed uninstall should print nothing, got: %s", stdout)
	}
}

func TestSkillShow_JSON(t *testing.T) {
	env, stdout, _ := newSkillEnv(true, false, "")
	if err := (&SkillShowCmd{}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got skillShowJSON
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("JSON: %v\n%s", err, stdout)
	}
	if got.Name != skill.Name || got.Description != skill.Description || got.Content != skill.Render() {
		t.Errorf("unexpected JSON: %+v", got)
	}
}

func TestSkillShow_AgentText(t *testing.T) {
	env, stdout, _ := newSkillEnv(false, false, "")
	if err := (&SkillShowCmd{Agent: "pi"}).Run(env); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if want := "# SKILL.md\n" + skill.Render(); stdout.String() != want {
		t.Errorf("output = %q, want %q", stdout, want)
	}
}

func TestSkillShow_UnknownAgent(t *testing.T) {
	env, stdout, _ := newSkillEnv(false, false, "")
	err := (&SkillShowCmd{Agent: "bogus"}).Run(env)
	if err == nil || !strings.Contains(err.Error(), `unknown agent "bogus"`) {
		t.Fatalf("expected unknown agent error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected output: %s", stdout)
	}
}
