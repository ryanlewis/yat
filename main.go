package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ryanlewis/yat/internal/cmd"
	"github.com/ryanlewis/yat/internal/config"
	"github.com/ryanlewis/yat/internal/skill"
	"golang.org/x/term"
)

type cli struct {
	Dir  string `help:"Path to items directory (overrides .yat.yaml)." type:"path" env:"YAT_DIR"`
	JSON bool   `help:"Output as JSON." short:"j"`

	// Dashboard runs when no command is given (bare `yat`).
	Dashboard cmd.DashboardCmd `cmd:"" default:"1" hidden:"" help:"Show in-progress and ready items at a glance."`

	Init     cmd.InitCmd     `cmd:"" help:"Initialize a new yat project in the current directory."`
	Ready    cmd.ReadyCmd    `cmd:"" help:"Show items ready to work on."`
	Next     cmd.NextCmd     `cmd:"" help:"Show the highest-priority ready item."`
	Show     cmd.ShowCmd     `cmd:"" help:"Show an item by ID."`
	Add      cmd.AddCmd      `cmd:"" help:"Create a new item."`
	Start    cmd.StartCmd    `cmd:"" help:"Start working on an item."`
	Complete cmd.CompleteCmd `cmd:"" help:"Mark an item as done."`
	Blocked  cmd.BlockedCmd  `cmd:"" help:"Show blocked items and their dependencies."`
	List     cmd.ListCmd     `cmd:"" help:"List all items grouped by status."`
	Status   cmd.StatusCmd   `cmd:"" help:"Show an overview of all items."`
	Graph    cmd.GraphCmd    `cmd:"" help:"Show the dependency graph."`
	Agents   cmd.AgentsCmd   `cmd:"" help:"Print the agent skill (alias for 'skill show')."`
	Skill    cmd.SkillCmd    `cmd:"" help:"Manage the bundled agent skill (Claude Code, Codex, Pi)."`
}

func main() {
	var c cli

	ctx := kong.Parse(&c,
		kong.Name("yat"),
		kong.Description("YAML tracker — a lightweight CLI for issue tracking with YAML.\n\n"+
			"Items are .md files with YAML frontmatter containing an id field.\n"+
			"Non-item markdown files are silently skipped.\n"+
			"Add \"yat: ignore\" to frontmatter to explicitly exclude a file.\n\n"+
			"Run yat with no command to see in-progress and ready items at a glance."),
		kong.UsageOnError(),
		kong.Vars{"skill_agents": skill.AgentNames()},
	)

	if ctx.Command() == "init" || ctx.Command() == "init <dir>" {
		if err := c.Init.Run(c.JSON, os.Stdout, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// agents and skill need no project, so they run before config loading.
	if ctx.Command() == "agents" || strings.HasPrefix(ctx.Command(), "skill ") {
		env := &cmd.SkillEnv{
			JSON:        c.JSON,
			Interactive: term.IsTerminal(int(os.Stdin.Fd())), //nolint:gosec // file descriptors fit in int
			Stdout:      os.Stdout,
			Stderr:      os.Stderr,
			Stdin:       os.Stdin,
		}
		if err := ctx.Run(env); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, cfgErr := config.LoadWithFallback(c.Dir)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", cfgErr)
		os.Exit(1)
	}

	dir := c.Dir
	if dir == "" {
		dir = cfg.Dir
	}

	if dir == "" {
		dir = "spec"
	}

	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		// Only a defaulted directory means "not in a project"; a configured one that is missing is an error.
		exitMissingDir(dir, ctx.Command() == "dashboard" && c.Dir == "" && cfg.Dir == "", c.JSON)
	}

	rc, err := newRunContext(ctx.Selected(), dir, c.JSON, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = ctx.Run(rc)
	rc.Unlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newRunContext loads the items for the selected command. A command that
// writes items loads them under the items directory's lock, unless the project
// is read-only and it will refuse to write.
func newRunContext(selected *kong.Node, dir string, jsonOutput bool, cfg *config.Config) (*cmd.RunContext, error) {
	if cfg.ReadOnly || selected == nil || !selected.Target.CanAddr() {
		return cmd.NewRunContext(dir, jsonOutput, cfg)
	}

	target := selected.Target.Addr().Interface()
	if _, ok := target.(cmd.ItemWriter); !ok {
		return cmd.NewRunContext(dir, jsonOutput, cfg)
	}

	// add reads a piped body first, so a slow pipe does not hold the lock.
	if add, ok := target.(*cmd.AddCmd); ok {
		if err := add.ReadBody(); err != nil {
			return nil, err
		}
	}

	return cmd.NewLockedRunContext(dir, jsonOutput, cfg)
}

// exitMissingDir exits when the items directory does not exist: 0 with an
// onboarding hint when onboard is set, 1 with an error otherwise.
func exitMissingDir(dir string, onboard, jsonOutput bool) {
	if onboard {
		if err := cmd.WriteNoProject(os.Stdout, jsonOutput, dir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "error: items directory %q not found.\n", dir)
	fmt.Fprintf(os.Stderr,
		"Run 'yat init' to create a new project, or use --dir to specify a different directory.\n")
	os.Exit(1)
}
