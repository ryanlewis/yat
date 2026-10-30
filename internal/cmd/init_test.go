package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp changes into a temp directory and restores the original on cleanup.
func chdirTemp(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("Chdir cleanup: %v", err)
		}
	})

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	return dir
}

func TestInitCmd_Fresh(t *testing.T) {
	dir := chdirTemp(t)

	var buf bytes.Buffer
	stdin := strings.NewReader("spec\ny\ny\n") // dir prompt, confirm non-existent, proceed

	cmd := &InitCmd{}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "does not exist") {
		t.Errorf("expected directory warning, got:\n%s", out)
	}
	if !strings.Contains(out, "Initialized yat project") {
		t.Errorf("expected success message, got:\n%s", out)
	}

	// Verify .yat.yaml was created
	data, err := os.ReadFile(filepath.Join(dir, ".yat.yaml"))
	if err != nil {
		t.Fatalf("reading .yat.yaml: %v", err)
	}
	if !strings.Contains(string(data), "dir: spec") {
		t.Errorf(".yat.yaml content = %q, want dir: spec", string(data))
	}

	// Verify sample item was created
	samplePath := filepath.Join(dir, "spec", "example.md")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Error("expected sample item to be created")
	}
}

func TestInitCmd_WithArg(t *testing.T) {
	dir := chdirTemp(t)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "myitems"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify directory and config
	data, err := os.ReadFile(filepath.Join(dir, ".yat.yaml"))
	if err != nil {
		t.Fatalf("reading .yat.yaml: %v", err)
	}
	if !strings.Contains(string(data), "dir: myitems") {
		t.Errorf(".yat.yaml content = %q, want dir: myitems", string(data))
	}

	samplePath := filepath.Join(dir, "myitems", "example.md")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Error("expected sample item to be created")
	}
}

func TestInitCmd_ExistingItems(t *testing.T) {
	dir := chdirTemp(t)

	// Create items directory with existing files
	specDir := filepath.Join(dir, "spec")
	os.MkdirAll(specDir, 0o755)
	os.WriteFile(filepath.Join(specDir, "item1.md"), []byte("---\nid: TK-001\n---\n"), 0o644)
	os.WriteFile(filepath.Join(specDir, "item2.md"), []byte("---\nid: TK-002\n---\n"), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "2 existing items") {
		t.Errorf("expected existing items message, got:\n%s", out)
	}

	// Verify no sample item was created
	samplePath := filepath.Join(dir, "spec", "example.md")
	if _, err := os.Stat(samplePath); !os.IsNotExist(err) {
		t.Error("expected no sample item when existing items present")
	}
}

func TestInitCmd_AlreadyInitialized(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, ".yat.yaml"), []byte("dir: spec\n"), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "spec"}
	err := cmd.Run(false, &buf, &buf, stdin)
	if err == nil {
		t.Fatal("expected error for already initialized")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("expected 'already initialized' in error, got: %v", err)
	}
}

func TestInitCmd_Declined(t *testing.T) {
	dir := chdirTemp(t)

	var buf bytes.Buffer
	stdin := strings.NewReader("n\n")

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "Aborted") {
		t.Errorf("expected 'Aborted' message, got:\n%s", buf.String())
	}

	// Verify nothing was created
	if _, err := os.Stat(filepath.Join(dir, ".yat.yaml")); !os.IsNotExist(err) {
		t.Error("expected no .yat.yaml when declined")
	}
}

func TestInitCmd_JSON(t *testing.T) {
	chdirTemp(t)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(true, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// JSON object is the last thing written; find it after the prompt output.
	output := buf.String()
	jsonStart := strings.LastIndex(output, "{")
	if jsonStart < 0 {
		t.Fatalf("no JSON found in output:\n%s", output)
	}

	var result initJSON
	if err := json.Unmarshal([]byte(output[jsonStart:]), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput:\n%s", err, output)
	}

	if result.ConfigFile != ".yat.yaml" {
		t.Errorf("ConfigFile = %q, want .yat.yaml", result.ConfigFile)
	}
	if result.Dir != defaultDir {
		t.Errorf("Dir = %q, want %s", result.Dir, defaultDir)
	}
	if !result.Created {
		t.Error("Created = false, want true")
	}
	if result.SampleItem != filepath.Join("spec", "example.md") {
		t.Errorf("SampleItem = %q, want spec/example.md", result.SampleItem)
	}
}

func TestInitCmd_EmptyDirGetsSample(t *testing.T) {
	dir := chdirTemp(t)

	// Create an empty items directory
	os.MkdirAll(filepath.Join(dir, "spec"), 0o755)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Should create sample item even though dir exists (but is empty)
	samplePath := filepath.Join(dir, "spec", "example.md")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Error("expected sample item in empty existing directory")
	}
}

func TestInitCmd_AlreadyInitialized_LocalYaml(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, ".yat.local.yaml"), []byte("dir: spec\n"), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n")

	cmd := &InitCmd{Dir: "spec"}
	err := cmd.Run(false, &buf, &buf, stdin)
	if err == nil {
		t.Fatal("expected error for already initialized")
	}
	if !strings.Contains(err.Error(), ".yat.local.yaml") {
		t.Errorf("expected config file name in error, got: %v", err)
	}
}

func TestInitCmd_AgentFiles_ClaudeMD(t *testing.T) {
	dir := chdirTemp(t)

	original := "# My Project\n\nSome existing content.\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(original), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "append yat instructions to CLAUDE.md") {
		t.Errorf("expected plan to mention CLAUDE.md, got:\n%s", out)
	}
	if !strings.Contains(out, "updated: CLAUDE.md") {
		t.Errorf("expected result to mention CLAUDE.md, got:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Error("original content was not preserved")
	}
	if !strings.Contains(content, "## yat (issue tracker)") {
		t.Error("expected yat instructions in CLAUDE.md")
	}
	for _, want := range []string{"yat skill show", "yat skill install claude --path .claude/skills/yat", "yat next", "yat start <id>", "yat complete <id>"} {
		if !strings.Contains(content, want) {
			t.Errorf("pointer is missing %q", want)
		}
	}
	if strings.Contains(content, "## Safety") {
		t.Error("full skill text was appended instead of a pointer")
	}
	if n := strings.Count(content, "\n"); n > len(strings.Split(original, "\n"))+10 {
		t.Errorf("pointer is too long: %d lines", n)
	}
}

func TestInitCmd_AgentFiles_AgentsMD(t *testing.T) {
	dir := chdirTemp(t)

	original := "# Agents\n"
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(original), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, original) {
		t.Error("original content was not preserved")
	}
	if !strings.Contains(content, "## yat (issue tracker)") {
		t.Error("expected yat instructions in AGENTS.md")
	}
}

func TestInitCmd_AgentFiles_Both(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents\n"), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "items"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !strings.Contains(string(data), "## yat (issue tracker)") {
			t.Errorf("expected yat instructions in %s", name)
		}
	}
}

func TestInitCmd_AgentFiles_JSON(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude\n"), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(true, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := buf.String()
	jsonStart := strings.LastIndex(output, "{")
	if jsonStart < 0 {
		t.Fatalf("no JSON found in output:\n%s", output)
	}

	var result initJSON
	if err := json.Unmarshal([]byte(output[jsonStart:]), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result.AgentFiles) != 1 || result.AgentFiles[0] != "CLAUDE.md" {
		t.Errorf("AgentFiles = %v, want [CLAUDE.md]", result.AgentFiles)
	}
}

func TestInitCmd_AgentFiles_Declined(t *testing.T) {
	dir := chdirTemp(t)

	original := "# Claude\n"
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(original), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("n\ny\n") // decline agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "append yat instructions") {
		t.Errorf("plan should not mention agent files when declined, got:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if string(data) != original {
		t.Errorf("CLAUDE.md was modified after decline: %q", string(data))
	}
}

func TestPromptDir_Default(t *testing.T) {
	chdirTemp(t)

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader("\ny\n")) // default "spec", confirm creation

	got := promptDir(scanner, &buf)
	if got != defaultDir {
		t.Errorf("promptDir = %q, want %s", got, defaultDir)
	}

	if !strings.Contains(buf.String(), "does not exist") {
		t.Error("expected non-existence warning for missing directory")
	}
}

func TestPromptDir_Custom(t *testing.T) {
	chdirTemp(t)

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader("myitems\ny\n")) // custom dir, confirm creation

	got := promptDir(scanner, &buf)
	if got != "myitems" {
		t.Errorf("promptDir = %q, want myitems", got)
	}
}

func TestPromptDir_ExistingDir(t *testing.T) {
	dir := chdirTemp(t)
	os.MkdirAll(filepath.Join(dir, "spec"), 0o755)

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader("\n")) // default "spec", no confirmation needed

	got := promptDir(scanner, &buf)
	if got != defaultDir {
		t.Errorf("promptDir = %q, want %s", got, defaultDir)
	}

	if strings.Contains(buf.String(), "does not exist") {
		t.Error("unexpected non-existence warning for existing directory")
	}
}

func TestPromptDir_TrailingSlash(t *testing.T) {
	dir := chdirTemp(t)
	os.MkdirAll(filepath.Join(dir, "spec"), 0o755)

	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader("spec/\n"))

	got := promptDir(scanner, &buf)
	if got != defaultDir {
		t.Errorf("promptDir = %q, want %s (cleaned)", got, defaultDir)
	}
}

func TestPromptDir_Retry(t *testing.T) {
	dir := chdirTemp(t)
	os.MkdirAll(filepath.Join(dir, "items"), 0o755)

	var buf bytes.Buffer
	// "backlog" doesn't exist → decline → "items" exists → accepted
	scanner := bufio.NewScanner(strings.NewReader("backlog\nn\nitems\n"))

	got := promptDir(scanner, &buf)
	if got != "items" {
		t.Errorf("promptDir = %q, want items", got)
	}

	out := buf.String()
	if !strings.Contains(out, `"backlog" does not exist`) {
		t.Errorf("expected warning about backlog, got:\n%s", out)
	}
}

func TestShowDirSuggestions(t *testing.T) {
	dir := chdirTemp(t)

	os.MkdirAll(filepath.Join(dir, "spec"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".hidden"), 0o755)
	os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	os.WriteFile(filepath.Join(dir, "spec", "item1.md"), []byte("test"), 0o644)
	os.WriteFile(filepath.Join(dir, "spec", "item2.md"), []byte("test"), 0o644)

	var buf bytes.Buffer
	showDirSuggestions(&buf)

	out := buf.String()
	if !strings.Contains(out, "spec/ (2 items)") {
		t.Errorf("expected spec with item count, got: %s", out)
	}
	if !strings.Contains(out, "docs/") {
		t.Errorf("expected docs/, got: %s", out)
	}
	if strings.Contains(out, ".hidden") {
		t.Error("should not list hidden directories")
	}
}

func TestShowDirSuggestions_SingleItem(t *testing.T) {
	dir := chdirTemp(t)

	os.MkdirAll(filepath.Join(dir, "spec"), 0o755)
	os.WriteFile(filepath.Join(dir, "spec", "item.md"), []byte("test"), 0o644)

	var buf bytes.Buffer
	showDirSuggestions(&buf)

	if !strings.Contains(buf.String(), "spec/ (1 item)") {
		t.Errorf("expected singular 'item', got: %s", buf.String())
	}
}

func TestShowDirSuggestions_Empty(t *testing.T) {
	chdirTemp(t)

	var buf bytes.Buffer
	showDirSuggestions(&buf)

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty directory, got: %s", buf.String())
	}
}

func TestInitCmd_AgentFiles_SkipsExistingBlock(t *testing.T) {
	// A file with the short pointer (current) or the full skill text (older yat)
	// must not get a second block. Both open with the same heading.
	blocks := map[string]string{
		"pointer":    agentPointer,
		"full skill": "\n## yat (issue tracker)\n\nyat is a CLI issue tracker.\n\n### Safety\n\n- Safe to run freely.\n",
	}

	for name, block := range blocks {
		t.Run(name, func(t *testing.T) {
			dir := chdirTemp(t)

			original := "# Claude\n" + block
			os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(original), 0o644)
			os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# Agents\n"), 0o644)

			var buf bytes.Buffer
			stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

			cmd := &InitCmd{Dir: "spec"}
			if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
				t.Fatalf("Run: %v", err)
			}

			out := buf.String()
			if !strings.Contains(out, "Add yat instructions to AGENTS.md? [Y/n]") {
				t.Errorf("expected prompt for AGENTS.md only, got:\n%s", out)
			}
			if !strings.Contains(out, "CLAUDE.md already has yat instructions, skipping.") {
				t.Errorf("expected skip note for CLAUDE.md, got:\n%s", out)
			}

			data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
			if err != nil {
				t.Fatalf("reading CLAUDE.md: %v", err)
			}
			if string(data) != original {
				t.Errorf("CLAUDE.md was modified:\n%s", data)
			}

			data, err = os.ReadFile(filepath.Join(dir, "AGENTS.md"))
			if err != nil {
				t.Fatalf("reading AGENTS.md: %v", err)
			}
			if strings.Count(string(data), "## yat (issue tracker)") != 1 {
				t.Errorf("expected one yat block in AGENTS.md, got:\n%s", data)
			}
		})
	}
}

func TestInitCmd_AgentFiles_AllSkippedNoPrompt(t *testing.T) {
	dir := chdirTemp(t)

	original := "# Claude\n" + agentPointer
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(original), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n") // accept plan; no agent prompt expected

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Add yat instructions") {
		t.Errorf("did not expect an agent prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "Initialized yat project.") {
		t.Errorf("expected init to proceed, got:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if string(data) != original {
		t.Errorf("CLAUDE.md was modified:\n%s", data)
	}
}

func TestInitCmd_AgentFiles_SymlinkAppendedOnce(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude\n"), 0o644)
	if err := os.Symlink("CLAUDE.md", filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Skipf("symlink: %v", err)
	}

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if n := strings.Count(string(data), "## yat (issue tracker)"); n != 1 {
		t.Errorf("expected one yat block, got %d:\n%s", n, data)
	}
}

func TestInitCmd_AgentFiles_UnreadableStillOffered(t *testing.T) {
	dir := chdirTemp(t)

	path := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(path, []byte("# Claude\n"), 0o200)
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("file is readable anyway (running as root?)")
	}

	var buf bytes.Buffer
	stdin := strings.NewReader("y\ny\n") // accept agent files, accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(false, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !strings.Contains(buf.String(), "updated: CLAUDE.md") {
		t.Errorf("expected CLAUDE.md to be updated, got:\n%s", buf.String())
	}
}

func TestInitCmd_AgentFiles_SkippedJSON(t *testing.T) {
	dir := chdirTemp(t)

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# Claude\n"+agentPointer), 0o644)

	var buf bytes.Buffer
	stdin := strings.NewReader("y\n") // accept plan

	cmd := &InitCmd{Dir: "spec"}
	if err := cmd.Run(true, &buf, &buf, stdin); err != nil {
		t.Fatalf("Run: %v", err)
	}

	output := buf.String()
	jsonStart := strings.LastIndex(output, "{")
	if jsonStart < 0 {
		t.Fatalf("no JSON found in output:\n%s", output)
	}

	var result initJSON
	if err := json.Unmarshal([]byte(output[jsonStart:]), &result); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	if len(result.Skipped) != 1 || result.Skipped[0] != "CLAUDE.md" {
		t.Errorf("Skipped = %v, want [CLAUDE.md]", result.Skipped)
	}
	if len(result.AgentFiles) != 0 {
		t.Errorf("AgentFiles = %v, want none", result.AgentFiles)
	}
}

func TestHasAgentHeading_CodeFence(t *testing.T) {
	const h = "## yat (issue tracker)\n"

	tests := map[string]struct {
		content string
		want    bool
	}{
		"plain":              {"# A\n\n" + h, true},
		"backtick fence":     {"```md\n" + h + "```\n", false},
		"tilde fence":        {"~~~\n" + h + "~~~\n", false},
		"long fence":         {"````\n```\n" + h + "````\n", false},
		"other char closes":  {"```\n~~~\n" + h + "```\n", false},
		"after closed fence": {"```\nx\n```\n" + h, true},
		"unclosed fence":     {"```\n" + h, false},
		"info on close":      {"```\n```js\n" + h, false},
		"indented fence":     {"   ```\n" + h + "   ```\n", false},
		"no heading":         {"```\nx\n```\n", false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := hasAgentHeading(tc.content); got != tc.want {
				t.Errorf("hasAgentHeading = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHasAgentHeading_InlineBackticksNotFence(t *testing.T) {
	// A backtick fence's info string cannot hold a backtick: this is inline code.
	if !hasAgentHeading("```x``` is inline\n\n## yat (issue tracker)\n") {
		t.Error("inline ``` opened a fence; real heading missed")
	}
}
