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
	minFenceRun    = 3 // shortest run of backticks or tildes that opens a fence
	maxFenceIndent = 3 // a fence indented further is code, not a fence
)

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

// Run executes the init command. It takes jsonMode, stdout, stderr and stdin
// directly because it runs before NewRunContext (the items directory may not
// exist yet). In JSON mode the prompts and plan go to stderr, so stdout holds
// only the JSON result.
func (i *InitCmd) Run(jsonMode bool, stdout, stderr io.Writer, stdin io.Reader) error {
	if existing := existingConfig(); existing != "" {
		return fmt.Errorf("yat is already initialized (%s exists)", existing)
	}

	msgs := stdout
	if jsonMode {
		msgs = stderr
	}

	dir, ask, err := i.chooseDir(msgs, stdin)
	if err != nil {
		return err
	}
	defer ask.close()

	plan := buildInitPlan(dir)

	found, skipped := detectAgentFiles()
	for _, f := range skipped {
		fmt.Fprintf(msgs, "%s already has yat instructions, skipping.\n", f)
	}

	if len(found) > 0 {
		add, confirmErr := confirm(ask, fmt.Sprintf("Add yat instructions to %s? [Y/n]: ", strings.Join(found, ", ")))
		if confirmErr != nil {
			return confirmErr
		}

		if add {
			plan.agentFiles = found
		}
	}

	printPlan(msgs, plan)

	fmt.Fprintln(msgs)

	ok, err := confirm(ask, "Proceed? [Y/n]: ")
	if err != nil {
		return err
	}

	if !ok {
		fmt.Fprintln(msgs, "Aborted.")
		return nil
	}

	if err := executePlan(plan); err != nil {
		return err
	}

	return printResult(jsonMode, stdout, plan, skipped)
}

// chooseDir returns the items directory and the prompter that asks the rest of
// init's questions. Every answer goes through that one prompter: readline and
// bufio.Scanner both read ahead, so a second reader on the same input would
// lose answers sent together, as when piped or pasted.
func (i *InitCmd) chooseDir(stdout io.Writer, stdin io.Reader) (string, prompter, error) {
	scanner := bufio.NewScanner(stdin)

	switch {
	case i.Dir != "":
		dir, err := config.ExpandTilde(i.Dir)
		return dir, scanPrompter{stdout: stdout, scanner: scanner}, err
	case isTerminal(stdin):
		return promptDirInteractive(stdout, stdin)
	default:
		showDirSuggestions(stdout)
		return promptDir(scanner, stdout), scanPrompter{stdout: stdout, scanner: scanner}, nil
	}
}

// prompter asks a question and returns the next line of input.
type prompter interface {
	ask(prompt string) (string, error)
	close()
}

// scanPrompter prints each prompt to stdout and reads lines with one scanner.
type scanPrompter struct {
	stdout  io.Writer
	scanner *bufio.Scanner
}

func (p scanPrompter) ask(prompt string) (string, error) {
	fmt.Fprint(p.stdout, prompt)

	if p.scanner.Scan() {
		return p.scanner.Text(), nil
	}

	if err := p.scanner.Err(); err != nil {
		return "", err
	}

	return "", io.EOF
}

func (p scanPrompter) close() {}

// readlinePrompter asks every question through one readline instance.
type readlinePrompter struct {
	rl *readline.Instance
}

func (p readlinePrompter) ask(prompt string) (string, error) {
	p.rl.SetPrompt(prompt)

	line, err := p.rl.ReadLine()
	if errors.Is(err, readline.ErrInterrupt) {
		return "", errInterrupted
	}

	return line, err
}

func (p readlinePrompter) close() {
	_ = p.rl.Close()
}

var errInterrupted = errors.New("interrupted")

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

// hasAgentHeading reports whether content has the yat heading outside a code
// fence (``` or ~~~).
func hasAgentHeading(content string) bool {
	var fence string // run of the open fence, empty outside one

	for line := range strings.Lines(content) {
		line = strings.TrimRight(line, " \t\r\n")
		marker := fenceRun(line)

		switch {
		case fence != "":
			if marker != "" && marker[0] == fence[0] && len(marker) >= len(fence) &&
				strings.Trim(strings.TrimSpace(line), marker[:1]) == "" {
				fence = ""
			}
		case marker != "":
			fence = marker
		case line == agentHeading:
			return true
		}
	}

	return false
}

// fenceRun returns the leading run of three or more backticks or tildes on
// line (after up to three spaces), or "".
func fenceRun(line string) string {
	rest := strings.TrimLeft(line, " ")
	if len(line)-len(rest) > maxFenceIndent || rest == "" || (rest[0] != '`' && rest[0] != '~') {
		return ""
	}

	run := rest[:len(rest)-len(strings.TrimLeft(rest, rest[:1]))]
	if len(run) < minFenceRun {
		return ""
	}

	// A backtick fence's info string cannot hold a backtick: that is inline code.
	if run[0] == '`' && strings.Contains(rest[len(run):], "`") {
		return ""
	}

	return run
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

// promptDir asks for the items directory on scanner, which the caller goes on
// using for later questions.
func promptDir(scanner *bufio.Scanner, stdout io.Writer) string {
	ask := scanPrompter{stdout: stdout, scanner: scanner}

	for {
		line, err := ask.ask(fmt.Sprintf("Items directory [%s]: ", defaultDir))
		if err != nil {
			return defaultDir
		}

		dir := defaultDir
		if line = strings.TrimSpace(line); line != "" {
			dir = filepath.Clean(line)
		}

		dir, ok, err := resolveDir(dir, ask, stdout)
		if err != nil {
			return defaultDir
		}

		if ok {
			return dir
		}
	}
}

// resolveDir expands tildes, checks whether the directory exists, and if not,
// asks the user to confirm creation. Returns the resolved path and true if
// accepted, or ("", false) to re-prompt. The error is set only if the user
// interrupted the confirm.
func resolveDir(dir string, ask prompter, stdout io.Writer) (resolved string, ok bool, err error) {
	expanded, expandErr := config.ExpandTilde(dir)
	if expandErr != nil {
		fmt.Fprintf(stdout, "Cannot expand ~: %v\n", expandErr)
		return "", false, nil
	}
	dir = expanded

	if info, statErr := os.Stat(dir); statErr == nil && info.IsDir() {
		return dir, true, nil
	}

	fmt.Fprintf(stdout, "Directory %q does not exist and will be created.\n", dir)

	ok, err = confirm(ask, "Use this path? [Y/n]: ")

	return dir, ok, err
}

func showDirSuggestions(stdout io.Writer) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return
	}

	var parts []string

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || !isDirEntry(".", e) {
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

// promptDirInteractive asks for the items directory with tab completion. It
// returns the prompter for init's later questions, which the caller closes;
// on error the prompter is already closed.
func promptDirInteractive(stdout io.Writer, stdin io.Reader) (string, prompter, error) {
	showDirSuggestions(stdout)

	// No history: the confirms share this instance, and the up arrow at one
	// would fill in a directory name.
	rl, err := readline.NewFromConfig(&readline.Config{
		AutoComplete: dirAutoCompleter{},
		HistoryLimit: -1,
		Stdout:       stdout,
	})
	if err != nil {
		fmt.Fprintf(stdout, "Note: tab completion unavailable (%v)\n", err)
		scanner := bufio.NewScanner(stdin)

		return promptDir(scanner, stdout), scanPrompter{stdout: stdout, scanner: scanner}, nil
	}

	ask := readlinePrompter{rl: rl}

	dir, err := readDirInteractive(ask, stdout)
	if err != nil {
		ask.close()
		return "", nil, err
	}

	return dir, ask, nil
}

func readDirInteractive(ask prompter, stdout io.Writer) (string, error) {
	for {
		line, err := ask.ask(fmt.Sprintf("Items directory [%s]: ", defaultDir))
		if errors.Is(err, errInterrupted) {
			return "", err
		}

		if err != nil {
			return "", fmt.Errorf("reading input: %w", err)
		}

		dir := strings.TrimSpace(line)
		if dir == "" {
			dir = defaultDir
		}

		resolved, ok, err := resolveDir(filepath.Clean(dir), ask, stdout)
		if err != nil {
			return "", err
		}

		if ok {
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

// isDirEntry reports whether e, an entry in dir, is a directory or a symlink
// that resolves to one.
func isDirEntry(dir string, e fs.DirEntry) bool {
	if e.Type()&fs.ModeSymlink == 0 {
		return e.IsDir()
	}

	info, err := os.Stat(filepath.Join(dir, e.Name()))

	return err == nil && info.IsDir()
}

func dirSuffixes(dir, prefix, prepend string) [][]rune {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var candidates [][]rune

	for _, e := range entries {
		// Check the name before isDirEntry, which stats symlinks.
		if strings.HasPrefix(e.Name(), ".") || !strings.HasPrefix(e.Name(), prefix) || !isDirEntry(dir, e) {
			continue
		}

		suffix := e.Name()[len(prefix):]
		candidates = append(candidates, []rune(prepend+suffix+"/"))
	}

	return candidates
}

// confirm asks a yes/no question that defaults to yes. End of input is a no;
// an interrupt is returned as an error.
func confirm(ask prompter, prompt string) (bool, error) {
	line, err := ask.ask(prompt)
	if errors.Is(err, errInterrupted) {
		return false, err
	}

	if err != nil {
		return false, nil
	}

	line = strings.TrimSpace(strings.ToLower(line))

	return line == "" || line == "y" || line == "yes", nil
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
