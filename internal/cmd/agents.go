package cmd

import "fmt"

// AgentsCmd prints a usage guide for AI agents interacting with yat.
type AgentsCmd struct{}

const agentsText = `# yat — Agent Usage Guide

yat is a CLI issue tracker. Items are markdown files with YAML frontmatter.
yat builds a dependency graph and surfaces what to work on next.

## Workflow

1. Run ` + "`yat status`" + ` to see an overview of all items.
2. Run ` + "`yat next`" + ` to get the highest-priority ready item.
3. Run ` + "`yat start <id>`" + ` to mark it as in-progress.
4. Do the work described in the item body.
5. Run ` + "`yat complete <id>`" + ` to mark it as done.
6. Repeat from step 2.

## Commands

  yat status           Overview of all items (counts, points, what's ready).
  yat next             Show the single highest-priority item ready for work.
                       Includes the full item body with requirements.
  yat ready            List all items ready to work on (dependencies met).
  yat show <id>        Show full details of any item by ID.
  yat start <id>       Mark an item as in-progress.
  yat complete <id>    Mark an item as done. Shows newly unblocked items.
  yat blocked          Show items that can't start yet and what they wait on.
  yat graph            Show the dependency graph as topological layers.

All commands support --json for structured output.

## Key Concepts

- Items have a status: todo → in-progress → done.
- Items have a priority: Critical > High > Medium > Low.
- Items can depend on other items. A blocked item cannot be started
  until all its dependencies are done.
- ` + "`yat next`" + ` picks the highest-priority item from the ready set.
- ` + "`yat complete`" + ` tells you which items are newly unblocked.

## Tips

- Always check ` + "`yat next`" + ` rather than picking items yourself.
  It respects priority and dependency order.
- Use ` + "`yat show <id>`" + ` to read the full requirements before starting work.
- After completing an item, check the "Now unblocked" output to see
  what's available next.
- If the project is read-only, start/complete will return an error.
  Just do the work without updating status.
- Use --json when you need to parse output programmatically.
`

// Run prints the agent usage guide.
func (a *AgentsCmd) Run() error {
	fmt.Print(agentsText)
	return nil
}
