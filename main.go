package main

import (
	"fmt"
	"io"
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
	os.Exit(execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// execute runs yat with args and returns the exit code. Commands that load items
// still write through their RunContext, which uses os.Stdout, and add reads a
// piped body from os.Stdin.
func execute(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var c cli

	ctx, code, ok := parse(&c, args, stdout, stderr)
	if !ok {
		return code
	}

	if ctx.Command() == "init" || ctx.Command() == "init <dir>" {
		return report(stderr, c.Init.Run(c.JSON, stdout, stdin))
	}

	// agents and skill need no project, so they run before config loading.
	if ctx.Command() == "agents" || strings.HasPrefix(ctx.Command(), "skill ") {
		return report(stderr, ctx.Run(&cmd.SkillEnv{
			JSON:        c.JSON,
			Interactive: isTerminal(stdin),
			Stdout:      stdout,
			Stderr:      stderr,
			Stdin:       stdin,
		}))
	}

	return runProject(ctx, &c, stdout, stderr)
}

// kongExit carries the exit code kong asks for out of parse.
type kongExit int

// parse parses args into c. ok is false when kong has already handled them,
// as for --help or a usage error, and the run should end with code. After
// parse returns, kong's Exit is os.Exit again.
func parse(c *cli, args []string, stdout, stderr io.Writer) (ctx *kong.Context, code int, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			exit, isExit := r.(kongExit)
			if !isExit {
				panic(r)
			}
			ctx, code, ok = nil, int(exit), false
		}
	}()

	parser := kong.Must(c,
		kong.Name("yat"),
		kong.Description("YAML tracker — a lightweight CLI for issue tracking with YAML.\n\n"+
			"Items are .md files with YAML frontmatter containing an id field.\n"+
			"Non-item markdown files are silently skipped.\n"+
			"Add \"yat: ignore\" to frontmatter to explicitly exclude a file.\n\n"+
			"Run yat with no command to see in-progress and ready items at a glance."),
		kong.UsageOnError(),
		kong.Vars{"skill_agents": skill.AgentNames()},
		kong.Writers(stdout, stderr),
		kong.Exit(func(code int) { panic(kongExit(code)) }),
	)

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	// Only parse recovers kongExit, so a later Exit must not panic with it.
	parser.Exit = os.Exit

	return ctx, 0, true
}

// runProject runs a command that needs the project's items.
func runProject(ctx *kong.Context, c *cli, stdout, stderr io.Writer) int {
	cfg, err := config.LoadWithFallback(c.Dir)
	if err != nil {
		return report(stderr, err)
	}

	dir := itemsDir(c.Dir, cfg.Dir)

	if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
		// Only a defaulted directory means "not in a project"; a configured one that is missing is an error.
		onboard := ctx.Command() == "dashboard" && c.Dir == "" && cfg.Dir == ""
		return missingDir(stdout, stderr, dir, onboard, c.JSON)
	}

	rc, err := newRunContext(ctx.Selected(), dir, c.JSON, &cfg)
	if err != nil {
		return report(stderr, err)
	}

	err = ctx.Run(rc)
	rc.Unlock()

	return report(stderr, err)
}

// itemsDir returns the items directory: the --dir flag, else the config's
// dir, else spec.
func itemsDir(flagDir, configDir string) string {
	if flagDir != "" {
		return flagDir
	}

	if configDir != "" {
		return configDir
	}

	return "spec"
}

// report writes err, if any, to stderr and returns the exit code for it.
func report(stderr io.Writer, err error) int {
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	return 0
}

// isTerminal reports whether r is a terminal.
func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd())) //nolint:gosec // file descriptors fit in int
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

// missingDir handles an items directory that does not exist: exit code 0
// with an onboarding hint when onboard is set, 1 with an error otherwise.
func missingDir(stdout, stderr io.Writer, dir string, onboard, jsonOutput bool) int {
	if onboard {
		return report(stderr, cmd.WriteNoProject(stdout, jsonOutput, dir))
	}

	fmt.Fprintf(stderr, "error: items directory %q not found.\n", dir)
	fmt.Fprintf(stderr,
		"Run 'yat init' to create a new project, or use --dir to specify a different directory.\n")

	return 1
}
