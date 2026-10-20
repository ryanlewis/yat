package cmd

import (
	"bytes"
	"encoding/json"
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
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("empty dir should be removed, stat err = %v", err)
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
