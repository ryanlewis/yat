// Package skill bundles the yat agent skill and manages its
// installation into supported AI coding agents (e.g. Claude Code).
//
// The skill body is authored in a neutral Markdown source (SKILL.md),
// embedded into the binary. Each Agent adapter renders that source into
// on-disk files appropriate for its target (e.g. Claude Code's SKILL.md with
// YAML frontmatter).
package skill

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed SKILL.md
var body string

// Skill identity shared across agents. Individual agents may override these
// when their target requires it, but today every agent uses the defaults.
const (
	Name        = "yat"
	Description = "Use when working with yat items, YAML-tracked issues, or when asked what to work on next " +
		"in a project with a `spec/` items directory or a `.yat.yaml` file. Provides the `yat` CLI for " +
		"finding ready items, starting and completing them, and reading the dependency graph."
)

// Body returns the neutral skill source body (no frontmatter).
func Body() string { return body }

// Render returns a self-contained SKILL.md: shared frontmatter + body.
func Render() string {
	return fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s", Name, Description, body)
}

// skillFileName is the one file every agent installs today.
const skillFileName = "SKILL.md"

// sharedFiles is the SKILL.md payload every registered agent installs today.
// Agents that need extra files should compose this with their own additions.
var sharedFiles = map[string][]byte{
	skillFileName: []byte(Render()),
}

// tildeRule says whether an agent expands a leading tilde in its own config
// directory variable. We have to match the agent exactly: expanding where the
// agent does not — or leaving it alone where the agent expands — installs the
// skill somewhere the agent never looks.
type tildeRule bool

const (
	// verbatimTilde hands the value to the filesystem as given, which is what
	// Claude Code and Codex do. Claude Code goes further and refuses to start
	// unless $CLAUDE_CONFIG_DIR is absolute.
	verbatimTilde tildeRule = false
	// expandsTilde rewrites a leading "~" or "~/" to $HOME first, which is
	// what Pi does to $PI_CODING_AGENT_DIR before reading from it.
	expandsTilde tildeRule = true
)

// resolveAgentDir returns the skill directory for an agent whose config
// directory can be relocated with envVar, falling back to $HOME joined with
// fallback when the variable is unset or empty. Beyond the agent's own tilde
// rule the value is used exactly as given — no relative-path rewriting.
func resolveAgentDir(envVar string, tilde tildeRule, fallback ...string) (string, error) {
	home := os.Getenv("HOME")
	base := os.Getenv(envVar)
	switch {
	case base == "":
		if home == "" {
			return "", fmt.Errorf("cannot locate the skill directory: neither $%s nor $HOME is set", envVar)
		}
		base = filepath.Join(append([]string{home}, fallback...)...)
	case tilde == expandsTilde && home != "":
		base = expandHomeTilde(base, home)
	}
	return filepath.Join(base, "skills", Name), nil
}

// expandHomeTilde rewrites a leading "~" or "~/" to home. Anything else — a
// "~user" form, a bare relative path — is left alone, matching the expansion
// Pi applies to $PI_CODING_AGENT_DIR.
func expandHomeTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// Permissions for installed files, matching the rest of yat's writes.
const (
	dirPerm  os.FileMode = 0o750
	filePerm os.FileMode = 0o600
)

// Agent renders and locates the skill for a particular AI coding agent.
type Agent interface {
	Name() string
	DefaultDir() (string, error)
	Files() map[string][]byte
}

// registry holds every supported agent. Adding an agent is one new adapter
// file plus one entry here.
var registry = index(claudeAgent{}, codexAgent{}, piAgent{})

func index(agents ...Agent) map[string]Agent {
	m := make(map[string]Agent, len(agents))
	for _, a := range agents {
		m[a.Name()] = a
	}
	return m
}

// Lookup returns the agent adapter with the given name.
func Lookup(name string) (Agent, error) {
	a, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q (supported: %s)", name, AgentNames())
	}
	return a, nil
}

// Agents returns the registered agents, sorted by name.
func Agents() []Agent {
	out := make([]Agent, 0, len(registry))
	for _, a := range registry {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// AgentNames returns a comma-separated list of registered agent names.
func AgentNames() string {
	agents := Agents()
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name()
	}
	return strings.Join(names, ", ")
}

// ownerHeader starts every file yat renders. A file without it was not
// written by yat, so install and uninstall leave it alone.
var ownerHeader = []byte("---\nname: " + Name + "\n")

// fileState says whether name is present under dir and, if so, whether yat
// wrote it.
func fileState(dir, name string) (present, owned bool) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false, false
	}
	return true, bytes.HasPrefix(data, ownerHeader)
}

// Exists reports whether any of the agent's files written by yat are present
// in dir.
func Exists(a Agent, dir string) bool {
	return len(InstalledFiles(a, dir)) > 0
}

// InstalledFiles returns the agent's files present under dir that yat wrote,
// sorted for stable output.
func InstalledFiles(a Agent, dir string) []string {
	var found []string
	for name := range a.Files() {
		if present, owned := fileState(dir, name); present && owned {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	return found
}

// ForeignFiles returns the files under dir that share a name with one of the
// agent's files but were not written by yat, sorted for stable output.
func ForeignFiles(a Agent, dir string) []string {
	var found []string
	for name := range a.Files() {
		if present, owned := fileState(dir, name); present && !owned {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	return found
}

// Install writes the agent's rendered files to dir, creating it if needed,
// and overwrites any existing files.
func Install(a Agent, dir string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	for name, content := range a.Files() {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
			return err
		}
		if err := os.WriteFile(path, content, filePerm); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes the agent's files that yat wrote from dir, leaving any
// other file alone. If the directory is empty afterwards, it is also removed.
func Uninstall(a Agent, dir string) error {
	for _, name := range InstalledFiles(a, dir) {
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
	return nil
}
