package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/ryanlewis/yat/internal/cmd"
	"github.com/ryanlewis/yat/internal/config"
)

type cli struct {
	Dir  string `help:"Path to items directory (overrides .yat.yaml)." type:"path" env:"YAT_DIR"`
	JSON bool   `help:"Output as JSON." short:"j"`

	Init     cmd.InitCmd     `cmd:"" help:"Initialize a new yat project in the current directory."`
	Ready    cmd.ReadyCmd    `cmd:"" help:"Show items ready to work on."`
	Next     cmd.NextCmd     `cmd:"" help:"Show the highest-priority ready item."`
	Show     cmd.ShowCmd     `cmd:"" help:"Show an item by ID."`
	Start    cmd.StartCmd    `cmd:"" help:"Start working on an item."`
	Complete cmd.CompleteCmd `cmd:"" help:"Mark an item as done."`
	Blocked  cmd.BlockedCmd  `cmd:"" help:"Show blocked items and their dependencies."`
	Status   cmd.StatusCmd   `cmd:"" help:"Show an overview of all items."`
	Graph    cmd.GraphCmd    `cmd:"" help:"Show the dependency graph."`
	Agents   cmd.AgentsCmd   `cmd:"" help:"Print a usage guide for AI agents."`
}

func main() {
	var c cli

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "yat: no command given. Run 'yat --help' for usage.\n")
		os.Exit(0)
	}

	ctx := kong.Parse(&c,
		kong.Name("yat"),
		kong.Description("YAML tracker — a lightweight CLI for issue tracking with YAML.\n\n"+
			"Items are .md files with YAML frontmatter containing an id field.\n"+
			"Non-item markdown files are silently skipped.\n"+
			"Add \"yat: ignore\" to frontmatter to explicitly exclude a file."),
		kong.UsageOnError(),
	)

	if ctx.Command() == "init" || ctx.Command() == "init <dir>" {
		if err := c.Init.Run(c.JSON, os.Stdout, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if ctx.Command() == "agents" {
		if err := c.Agents.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg, cfgErr := config.Load()
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
		fmt.Fprintf(os.Stderr, "error: items directory %q not found.\n", dir)
		fmt.Fprintf(os.Stderr,
			"Run 'yat init' to create a new project, or use --dir to specify a different directory.\n")
		os.Exit(1)
	}

	rc, err := cmd.NewRunContext(dir, c.JSON, &cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := ctx.Run(rc); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
