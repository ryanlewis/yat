package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ryanlewis/yat/internal/skill"
)

// SkillEnv carries the I/O the skill commands need. They run before config
// loading (no project is required), so they bypass RunContext.
type SkillEnv struct {
	JSON        bool
	Interactive bool
	Stdout      io.Writer
	Stderr      io.Writer
	Stdin       io.Reader
}

func (e *SkillEnv) writeJSON(v any) error {
	enc := json.NewEncoder(e.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(v)
}

// canPrompt reports whether a confirmation prompt can be shown. JSON mode
// never prompts, so scripted callers cannot block on stdin.
func (e *SkillEnv) canPrompt() bool {
	return e.Interactive && !e.JSON
}

// confirm asks a yes/no question on stderr. Anything but y/yes is a no.
func (e *SkillEnv) confirm(question string) bool {
	fmt.Fprintf(e.Stderr, "%s [y/N]: ", question)

	scanner := bufio.NewScanner(e.Stdin)
	if !scanner.Scan() {
		return false
	}

	answer := strings.TrimSpace(strings.ToLower(scanner.Text()))

	return answer == "y" || answer == "yes"
}

// SkillCmd manages the bundled agent skill.
type SkillCmd struct {
	Install   SkillInstallCmd   `cmd:"" help:"Install the bundled skill for an AI coding agent."`
	Uninstall SkillUninstallCmd `cmd:"" help:"Remove the bundled skill for an AI coding agent."`
	Show      SkillShowCmd      `cmd:"" help:"Print the skill (default: the rendered SKILL.md)."`
	List      SkillListCmd      `cmd:"" help:"List supported agents and where the skill is installed."`
}

// SkillInstallCmd installs the skill for one agent.
type SkillInstallCmd struct {
	Agent string `arg:"" help:"Target agent (${skill_agents})."`
	Path  string `help:"Install into this directory instead of the agent's default." type:"path"`
	Yes   bool   `help:"Overwrite an existing install without prompting." short:"y"`
}

type skillInstallJSON struct {
	Agent       string   `json:"agent"`
	Dir         string   `json:"dir"`
	Files       []string `json:"files"`
	Overwritten bool     `json:"overwritten"`
}

// Run executes the skill install command.
func (c *SkillInstallCmd) Run(env *SkillEnv) error {
	agent, dir, err := resolveSkillTarget(c.Agent, c.Path)
	if err != nil {
		return err
	}

	if foreign := skill.ForeignFiles(agent, dir); len(foreign) > 0 {
		return fmt.Errorf("%s in %s was not written by yat; refusing to overwrite it",
			strings.Join(foreign, ", "), dir)
	}

	exists := skill.Exists(agent, dir)
	if exists && !c.Yes {
		if !env.canPrompt() {
			return fmt.Errorf("skill already installed at %s; pass -y to overwrite", dir)
		}

		if !env.confirm(fmt.Sprintf("Skill already installed at %s. Overwrite?", dir)) {
			fmt.Fprintln(env.Stdout, "Aborted.")
			return nil
		}
	}

	if err := skill.Install(agent, dir); err != nil {
		return err
	}

	if env.JSON {
		return env.writeJSON(skillInstallJSON{
			Agent:       agent.Name(),
			Dir:         dir,
			Files:       fileNames(agent),
			Overwritten: exists,
		})
	}

	fmt.Fprintf(env.Stdout, "Installed %s skill to %s\n", agent.Name(), dir)

	return nil
}

// SkillUninstallCmd removes the skill's own files for one agent.
type SkillUninstallCmd struct {
	Agent string `arg:"" help:"Target agent (${skill_agents})."`
	Path  string `help:"Uninstall from this directory instead of the agent's default." type:"path"`
	Yes   bool   `help:"Uninstall without prompting." short:"y"`
}

type skillUninstallJSON struct {
	Agent   string   `json:"agent"`
	Dir     string   `json:"dir"`
	Removed []string `json:"removed"`
}

// Run executes the skill uninstall command.
func (c *SkillUninstallCmd) Run(env *SkillEnv) error {
	agent, dir, err := resolveSkillTarget(c.Agent, c.Path)
	if err != nil {
		return err
	}

	present := skill.InstalledFiles(agent, dir)
	if len(present) == 0 {
		return fmt.Errorf("no %s skill installed at %s", agent.Name(), dir)
	}

	if !c.Yes {
		if !env.canPrompt() {
			return fmt.Errorf("refusing to uninstall without confirmation; pass -y to confirm")
		}

		fmt.Fprintf(env.Stderr, "Will remove %d file(s) from %s:\n", len(present), dir)
		for _, f := range present {
			fmt.Fprintf(env.Stderr, "  - %s\n", f)
		}

		if !env.confirm(fmt.Sprintf("Remove %s skill at %s?", agent.Name(), dir)) {
			fmt.Fprintln(env.Stdout, "Aborted.")
			return nil
		}
	}

	// Only the agent's own skills/yat directory is yat's to remove; a --path
	// directory belongs to the user even when yat's files were all it held.
	if err := skill.Uninstall(agent, dir, c.Path == ""); err != nil {
		return err
	}

	if env.JSON {
		return env.writeJSON(skillUninstallJSON{Agent: agent.Name(), Dir: dir, Removed: present})
	}

	fmt.Fprintf(env.Stdout, "Removed %s skill from %s\n", agent.Name(), dir)

	return nil
}

// SkillShowCmd prints the skill.
type SkillShowCmd struct {
	Agent string `arg:"" optional:"" help:"Show the files installed for this agent (${skill_agents})."`
}

type skillShowJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

type skillShowAgentJSON struct {
	Agent string            `json:"agent"`
	Files map[string]string `json:"files"`
}

// Run executes the skill show command.
func (c *SkillShowCmd) Run(env *SkillEnv) error {
	if c.Agent == "" {
		if env.JSON {
			return env.writeJSON(skillShowJSON{
				Name:        skill.Name,
				Description: skill.Description,
				Content:     skill.Render(),
			})
		}

		fmt.Fprint(env.Stdout, skill.Render())

		return nil
	}

	agent, err := skill.Lookup(c.Agent)
	if err != nil {
		return err
	}

	files := agent.Files()

	if env.JSON {
		out := skillShowAgentJSON{Agent: agent.Name(), Files: make(map[string]string, len(files))}
		for name, content := range files {
			out.Files[name] = string(content)
		}

		return env.writeJSON(out)
	}

	for _, name := range fileNames(agent) {
		fmt.Fprintf(env.Stdout, "# %s\n%s", name, files[name])
	}

	return nil
}

// SkillListCmd lists the supported agents and their install state.
type SkillListCmd struct{}

type skillListJSON struct {
	Agent     string `json:"agent"`
	Dir       string `json:"dir,omitempty"`
	Installed bool   `json:"installed"`
	Error     string `json:"error,omitempty"`
}

// Run executes the skill list command.
func (c *SkillListCmd) Run(env *SkillEnv) error {
	agents := skill.Agents()
	rows := make([]skillListJSON, 0, len(agents))

	for _, a := range agents {
		dir, err := a.DefaultDir()
		if err != nil {
			// Report why rather than stat-ing a placeholder path, which would
			// probe the current directory.
			rows = append(rows, skillListJSON{Agent: a.Name(), Error: err.Error()})
			continue
		}

		rows = append(rows, skillListJSON{Agent: a.Name(), Dir: dir, Installed: skill.Exists(a, dir)})
	}

	if env.JSON {
		return env.writeJSON(rows)
	}

	for _, r := range rows {
		switch {
		case r.Error != "":
			fmt.Fprintf(env.Stdout, "%-8s (path unresolved: %s)\n", r.Agent, r.Error)
		case r.Installed:
			fmt.Fprintf(env.Stdout, "%-8s %s  (installed)\n", r.Agent, r.Dir)
		default:
			fmt.Fprintf(env.Stdout, "%-8s %s  (not installed)\n", r.Agent, r.Dir)
		}
	}

	fmt.Fprintf(env.Stdout, "\nInstall with: yat skill install <agent>  (agents: %s)\n", skill.AgentNames())

	return nil
}

func resolveSkillTarget(name, override string) (skill.Agent, string, error) {
	agent, err := skill.Lookup(name)
	if err != nil {
		return nil, "", err
	}

	if override != "" {
		return agent, override, nil
	}

	dir, err := agent.DefaultDir()
	if err != nil {
		return nil, "", err
	}

	return agent, dir, nil
}

func fileNames(a skill.Agent) []string {
	files := a.Files()
	names := make([]string, 0, len(files))

	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
