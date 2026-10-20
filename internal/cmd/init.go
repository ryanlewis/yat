package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ergochat/readline"
	"github.com/ryanlewis/yat/internal/config"
	"golang.org/x/term"
)

// InitCmd initializes a new yat project in the current directory.
type InitCmd struct {
	Dir string `arg:"" optional:"" help:"Items directory (default: spec, or prompted interactively)."`
}

type initJSON struct {
	ConfigFile string   `json:"config_file"`
	Dir        string   `json:"dir"`
	Created    bool     `json:"created"`
	SampleItem string   `json:"sample_item,omitempty"`
	AgentFiles []string `json:"agent_files,omitempty"`
	Skipped    []string `json:"skipped_agent_files,omitempty"`
}

var agentMDFiles = []string{"CLAUDE.md", "AGENTS.md"}

// agentPointer is appended to CLAUDE.md / AGENTS.md. It points at `yat skill`
// instead of copying the skill text, so the two cannot drift apart.
const agentPointer = `
## yat (issue tracker)

This project tracks items with yat. Run ` + "`yat skill show`" + ` for the full guide.
For a project-local install: ` + "`yat skill install claude --path .claude/skills/yat`" + `.
Core loop: ` + "`yat next`" + `, ` + "`yat start <id>`" + `, ` + "`yat complete <id>`" + `.
`

// agentHeading opens every yat block init has ever appended: the pointer above
// and the older full skill text, whose demoted title is the same line.
const agentHeading = "## yat (issue tracker)"

const (
	defaultDir                      = "spec"
	sampleItemFile                  = "example.md"
	initDirPermissions  fs.FileMode = 0o750
	initFilePermissions fs.FileMode = 0o600
)

const sampleItemContent = `---
id: EXAMPLE-001
title: My first item
type: Task
priority: Medium
status: todo
---

Replace this with a description of the work to be done.
`

// initPlan captures what the init command will do.
type initPlan struct {
	dir          string
	createDir    bool
	createSample bool
	samplePath   string
	mdCount      int
	agentFiles   []string
}

// Run executes the init command. It takes jsonMode, stdout, and stdin directly
// because it runs before NewRunContext (the items directory may not exist yet).
func (i *InitCmd) Run(jsonMode bool, stdout io.Writer, stdin io.Reader) error {
	if existing := existingConfig(); existing != "" {
		return fmt.Errorf("yat is already initialized (%s exists)", existing)
	}

	scanner := bufio.NewScanner(stdin)
	dir := i.Dir

	switch {
	case dir != "":
		expanded, err := config.ExpandTilde(dir)
		if err != nil {
			return err
		}
		dir = expanded
	case isTerminal(stdin):
		var err error
		dir, err = promptDirInteractive(stdout, stdin)
		if err != nil {
			return err
		}
	default:
		showDirSuggestions(stdout)
		dir = promptDir(scanner, stdout)
	}

	plan := buildInitPlan(dir)

	found, skipped := detectAgentFiles()
	for _, f := range skipped {
		fmt.Fprintf(stdout, "%s already has yat instructions, skipping.\n", f)
	}

	if len(found) > 0 {
		fmt.Fprintf(stdout, "Add yat instructions to %s? [Y/n]: ", strings.Join(found, ", "))
		if confirmPrompt(scanner) {
			plan.agentFiles = found
		}
	}

	printPlan(stdout, plan)

	fmt.Fprintf(stdout, "\nProceed? [Y/n]: ")

	if !confirmPrompt(scanner) {
		fmt.Fprintln(stdout, "Aborted.")
		return nil
	}

	if err := executePlan(plan); err != nil {
		return err
	}

	return printResult(jsonMode, stdout, plan, skipped)
}

func existingConfig() string {
	for _, name := range config.ConfigNames {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}

	return ""
}

func buildInitPlan(dir string) initPlan {
	p := initPlan{
		dir:        dir,
		samplePath: filepath.Join(dir, sampleItemFile),
	}

	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		p.createDir = true
	} else {
		p.mdCount = countMDFiles(dir)
	}

	p.createSample = p.mdCount == 0

	return p
}

// detectAgentFiles returns the agent files that exist and lack a yat block,
// and those that already have one and are left alone. A name that is the same
// file as an earlier one (e.g. AGENTS.md linked to CLAUDE.md) is dropped.
func detectAgentFiles() (found, skipped []string) {
	var seen []os.FileInfo

	for _, name := range agentMDFiles {
		info, err := os.Stat(name)
		if err != nil || slices.ContainsFunc(seen, func(s os.FileInfo) bool { return os.SameFile(s, info) }) {
			continue
		}

		seen = append(seen, info)

		// An unreadable file is still offered: the append may work, and if not
		// the user sees why.
		if data, err := os.ReadFile(name); err == nil && hasAgentHeading(string(data)) {
			skipped = append(skipped, name)
		} else {
			found = append(found, name)
		}
	}

	return found, skipped
}

func hasAgentHeading(content string) bool {
	for line := range strings.Lines(content) {
		if strings.TrimRight(line, " \t\r\n") == agentHeading {
			return true
		}
	}

	return false
}

func printPlan(stdout io.Writer, p initPlan) {
	if p.mdCount > 0 {
		fmt.Fprintf(stdout, "\nFound %d existing items in %s/\n", p.mdCount, p.dir)
	}

	fmt.Fprintf(stdout, "\nyat will:\n")
	fmt.Fprintf(stdout, "  create %s (dir: %s)\n", config.DefaultConfigFile, p.dir)

	if p.createDir {
		fmt.Fprintf(stdout, "  create %s/\n", p.dir)
	}

	if p.createSample {
		fmt.Fprintf(stdout, "  create %s\n", p.samplePath)
	}

	for _, f := range p.agentFiles {
		fmt.Fprintf(stdout, "  append yat instructions to %s\n", f)
	}
}

func executePlan(p initPlan) error {
	if p.createDir {
		if err := os.MkdirAll(p.dir, initDirPermissions); err != nil {
			return fmt.Errorf("creating directory %s: %w", p.dir, err)
		}
	}

	if p.createSample {
		if err := os.WriteFile(p.samplePath, []byte(sampleItemContent), initFilePermissions); err != nil {
			return fmt.Errorf("writing sample item: %w", err)
		}
	}

	configContent := fmt.Sprintf("dir: %s\n", p.dir)

	f, err := os.OpenFile(config.DefaultConfigFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, initFilePermissions)
	if err != nil {
		return fmt.Errorf("writing %s: %w", config.DefaultConfigFile, err)
	}

	_, writeErr := f.WriteString(configContent)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}

	if writeErr != nil {
		return fmt.Errorf("writing %s: %w", config.DefaultConfigFile, writeErr)
	}

	for _, af := range p.agentFiles {
		if err := appendAgentInstructions(af); err != nil {
			return err
		}
	}

	return nil
}

func appendAgentInstructions(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, initFilePermissions)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	_, writeErr := f.WriteString(agentPointer)
	if closeErr := f.Close(); writeErr == nil {
		writeErr = closeErr
	}

	if writeErr != nil {
		return fmt.Errorf("writing to %s: %w", path, writeErr)
	}

	return nil
}

func printResult(jsonMode bool, stdout io.Writer, p initPlan, skipped []string) error {
	if jsonMode {
		result := initJSON{
			ConfigFile: config.DefaultConfigFile,
			Dir:        p.dir,
			Created:    p.createDir,
			AgentFiles: p.agentFiles,
			Skipped:    skipped,
		}
		if p.createSample {
			result.SampleItem = p.samplePath
		}

		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(result)
	}

	fmt.Fprintln(stdout, "\nInitialized yat project.")
	fmt.Fprintf(stdout, "  config:  %s\n", config.DefaultConfigFile)

	if p.mdCount > 0 {
		fmt.Fprintf(stdout, "  items:   %s/ (%d existing items)\n", p.dir, p.mdCount)
	} else {
		fmt.Fprintf(stdout, "  items:   %s/\n", p.dir)
	}

	if p.createSample {
		fmt.Fprintf(stdout, "  example: %s\n", p.samplePath)
	}

	for _, f := range p.agentFiles {
		fmt.Fprintf(stdout, "  updated: %s\n", f)
	}

	return nil
}

func promptDir(scanner *bufio.Scanner, stdout io.Writer) string {
	for {
		fmt.Fprintf(stdout, "Items directory [%s]: ", defaultDir)

		dir := defaultDir
		if scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				dir = filepath.Clean(line)
			}
		} else {
			return defaultDir
		}

		dir, ok := resolveDir(dir, scanner, stdout)
		if ok {
			return dir
		}
	}
}

// resolveDir expands tildes, checks whether the directory exists, and if not,
// asks the user to confirm creation. Returns the resolved path and true if
// accepted, or ("", false) to re-prompt.
func resolveDir(dir string, scanner *bufio.Scanner, stdout io.Writer) (string, bool) {
	expanded, expandErr := config.ExpandTilde(dir)
	if expandErr != nil {
		fmt.Fprintf(stdout, "Cannot expand ~: %v\n", expandErr)
		return "", false
	}
	dir = expanded

	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir, true
	}

	fmt.Fprintf(stdout, "Directory %q does not exist and will be created.\n", dir)
	fmt.Fprintf(stdout, "Use this path? [Y/n]: ")

	return dir, confirmPrompt(scanner)
}

func showDirSuggestions(stdout io.Writer) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}

	var parts []string

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		name := e.Name()
		if count := countMDFiles(name); count > 0 {
			if count == 1 {
				parts = append(parts, fmt.Sprintf("%s/ (1 item)", name))
			} else {
				parts = append(parts, fmt.Sprintf("%s/ (%d items)", name, count))
			}
		} else {
			parts = append(parts, name+"/")
		}
	}

	if len(parts) > 0 {
		fmt.Fprintf(stdout, "Directories: %s\n", strings.Join(parts, "  "))
	}
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors fit in int
}

func promptDirInteractive(stdout io.Writer, stdin io.Reader) (string, error) {
	showDirSuggestions(stdout)

	rl, err := readline.NewFromConfig(&readline.Config{
		Prompt:       fmt.Sprintf("Items directory [%s]: ", defaultDir),
		AutoComplete: dirAutoCompleter{},
	})
	if err != nil {
		fmt.Fprintf(stdout, "Note: tab completion unavailable (%v)\n", err)
		scanner := bufio.NewScanner(stdin)
		return promptDir(scanner, stdout), nil
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				return "", fmt.Errorf("interrupted")
			}
			return "", fmt.Errorf("reading input: %w", err)
		}

		dir := strings.TrimSpace(line)
		if dir == "" {
			dir = defaultDir
		}

		dir = filepath.Clean(dir)

		scanner := bufio.NewScanner(stdin)
		if resolved, ok := resolveDir(dir, scanner, stdout); ok {
			return resolved, nil
		}
	}
}

type dirAutoCompleter struct{}

func (d dirAutoCompleter) Do(line []rune, pos int) (candidates [][]rune, length int) {
	input := string(line[:pos])

	if input == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, 0
		}

		return dirSuffixes(home, "", "/"), 0
	}

	searchDir := "."
	prefix := input

	if i := strings.LastIndex(input, "/"); i >= 0 {
		searchDir = input[:i]
		if searchDir == "" {
			searchDir = "/"
		}

		prefix = input[i+1:]
	}

	if expanded, err := config.ExpandTilde(searchDir); err == nil {
		searchDir = expanded
	}

	return dirSuffixes(searchDir, prefix, ""), len([]rune(prefix))
}

func dirSuffixes(dir, prefix, prepend string) [][]rune {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var candidates [][]rune

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		if strings.HasPrefix(e.Name(), prefix) {
			suffix := e.Name()[len(prefix):]
			candidates = append(candidates, []rune(prepend+suffix+"/"))
		}
	}

	return candidates
}

func confirmPrompt(scanner *bufio.Scanner) bool {
	if scanner.Scan() {
		line := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return line == "" || line == "y" || line == "yes"
	}

	return false
}

func countMDFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}

	return count
}
