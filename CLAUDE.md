# CLAUDE.md

## What is yat?

yat (YAML tracker) is a lightweight CLI issue tracker. Items (stories, tasks, spikes) are YAML-frontmattered markdown files. yat builds a dependency graph from these files and surfaces what to work on next.

## Commands

```sh
make build       # build binary to ./yat
make test        # go test -v -race -coverprofile=coverage.out ./...
make lint        # golangci-lint run
make coverage    # generate HTML coverage report
```

Run a single test:

```sh
go test -v -race -run TestReady_AllDraft ./internal/graph/
```

## Architecture

```
main.go              Kong CLI entry point, global flags (--dir, --json)
internal/
  item/              Parsing, types, and mutation of YAML-frontmattered .md files
    types.go         Item struct, Status/Priority enums with validation
    parse.go         Frontmatter extraction and field validation
    load.go          Recursive directory walk, bulk loading
    mutate.go        In-place status updates (rewrites frontmatter, preserves body)
  graph/             Dependency graph (Kahn's algorithm for topological layers)
    graph.go         Build, Ready, Blocked, WaitingOn, UnblockedBy, cycle detection
  cmd/               One file per command, all implement Run(*RunContext) error
    context.go       RunContext: loads items, builds graph, shared by all commands
```

**Data flow:** `main.go` → `cmd.NewRunContext(dir)` → `item.LoadAll(dir)` → `graph.Build(items)` → command `.Run(rc)`.

Every command supports text and `--json` output modes. Text goes through `tabwriter`; JSON through `encoding/json`.

## Conventions

- US English as enforced by `misspell` linter.
- Items are called "items" everywhere — not "specs", "issues", or "tickets".
- The `--dir` flag (env `YAT_DIR`, default `spec`) points to the items directory.
- Linting: golangci-lint with 28 linters enabled. Max line length 120, cyclomatic complexity 15, cognitive complexity 20.
- All new CLI output must support `--json` mode.
