# CLAUDE.md

## What is yat?

yat (YAML tracker) is a lightweight CLI issue tracker. Items (stories, tasks, spikes) are YAML-frontmattered markdown files. yat builds a dependency graph from these files and surfaces what to work on next.

## Commands

```sh
make build       # build binary to ./yat
make test        # go test -v -race -coverprofile=coverage.out ./...
make lint        # golangci-lint run
make coverage    # generate HTML coverage report
make clean       # remove binary, coverage files
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
  config/            Project config file loading (.yat.yaml / .yat.local.yaml)
    config.go        Load with directory traversal, tilde expansion
  cmd/               One file per command, all implement Run(*RunContext) error
    context.go       RunContext: loads items, builds graph, shared by all commands
```

**Data flow:** `main.go` → `cmd.NewRunContext(dir)` → `item.LoadAll(dir)` → `graph.Build(items)` → command `.Run(rc)`.

`init` and `agents` are special-cased in `main.go` — they run before config loading and bypass `RunContext` entirely.

Every command supports text and `--json` output modes. Text goes through `tabwriter`; JSON through `encoding/json`.

## Project Config (`.yat.yaml`)

yat looks for a project config file to set defaults (e.g. the items directory). On startup, if `--dir` is not passed, it searches from the current directory up to the filesystem root for the first match in this priority order:

1. `.yat.local.yaml` — per-machine overrides (gitignore this)
2. `.yat.local.yml`
3. `.yat.yaml` — shared project config (commit this)
4. `.yat.yml`

The config file is YAML with these fields:

```yaml
dir: path/to/items   # equivalent to --dir; supports ~ for home directory
readonly: true       # prevents yat start/complete from modifying item files
```

Resolution order for the items directory: `--dir` flag / `YAT_DIR` env → config file `dir` → default `spec`.

## Conventions

- US English as enforced by `misspell` linter.
- Items are called "items" everywhere — not "specs", "issues", or "tickets".
- The `--dir` flag (env `YAT_DIR`, default `spec`) points to the items directory.
- Linting: golangci-lint (see `.golangci.yml`). Max line length 120, cyclomatic complexity 15, cognitive complexity 20.
- All new CLI output must support `--json` mode.
