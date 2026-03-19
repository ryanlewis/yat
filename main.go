package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/ryanlewis/yat/internal/cmd"
)

type cli struct {
	Dir  string `help:"Path to items directory." type:"path" env:"YAT_DIR" default:"spec"`
	JSON bool   `help:"Output as JSON." short:"j"`

	Ready    cmd.ReadyCmd    `cmd:"" help:"Show items ready to work on."`
	Next     cmd.NextCmd     `cmd:"" help:"Show the highest-priority ready item."`
	Show     cmd.ShowCmd     `cmd:"" help:"Show an item by ID."`
	Start    cmd.StartCmd    `cmd:"" help:"Start working on an item."`
	Complete cmd.CompleteCmd `cmd:"" help:"Mark an item as done."`
	Blocked  cmd.BlockedCmd  `cmd:"" help:"Show blocked items and their dependencies."`
	Status   cmd.StatusCmd   `cmd:"" help:"Show an overview of all items."`
	Graph    cmd.GraphCmd    `cmd:"" help:"Show the dependency graph."`
}

func main() {
	var c cli

	ctx := kong.Parse(&c,
		kong.Name("yat"),
		kong.Description("YAML tracker — a lightweight CLI for issue tracking with YAML.\n\n"+
			"Items are .md files with YAML frontmatter containing an id field.\n"+
			"Non-item markdown files are silently skipped.\n"+
			"Add \"yat: ignore\" to frontmatter to explicitly exclude a file."),
		kong.UsageOnError(),
	)

	rc, err := cmd.NewRunContext(c.Dir, c.JSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := ctx.Run(rc); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
