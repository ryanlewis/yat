# yat

A lightweight CLI for issue tracking, backed by plain YAML-frontmattered markdown files.

Items (stories, tasks, spikes) live as `.md` files in a directory. Each file carries YAML frontmatter describing its ID, priority, dependencies, and status. `yat` reads these files, builds a dependency graph, and surfaces what to work on next.

## Install

```sh
go install github.com/ryanlewis/yat@latest
```

Or build from source:

```sh
make build
```

## Quick start

Initialize a project:

```sh
yat init
```

This creates a `.yat.yaml` config and a sample item in your items directory (default `spec/`). The interactive prompt offers tab completion for choosing a directory.

Items are loaded recursively, so you can organize them into subdirectories:

```
spec/
  stories/
    ST-001-auth-flow.md
  tasks/
    TK-001-setup-ci.md
  spikes/
    SP-001-evaluate-db.md
```

Each item is a markdown file with YAML frontmatter:

```markdown
---
id: TK-001
title: "Set up CI pipeline"
type: Task
priority: High
points: 3
dependencies: []
status: draft
phase: "1"
---

## Acceptance Criteria
- [ ] CI runs on every push
- [ ] Tests must pass before merge
```

Then use `yat` to track progress:

```sh
yat status           # overview of all items
yat list             # all items grouped by status
yat ready            # items with all dependencies met
yat next             # highest-priority ready item
yat show TK-001      # full details of an item
yat start TK-001     # mark as in-progress
yat complete TK-001  # mark as done
yat blocked          # items waiting on dependencies
yat graph            # dependency graph by layer
yat agents           # usage guide for AI agents
```

## Completing items

When you complete an item, yat tells you what's now available:

```
$ yat complete TK-001
Completed TK-001: Set up CI pipeline

Now unblocked:
  TK-002  Write integration tests

Still blocked (partial deps resolved):
  TK-003  Deploy to staging (waiting on: TK-002)
```

## Item fields

| Field          | Description                                 |
|----------------|---------------------------------------------|
| `id`           | Unique identifier (e.g. `ST-001`, `TK-002`) |
| `title`        | Short description                           |
| `type`         | `Story`, `Task`, or `Spike`                 |
| `priority`     | `Critical`, `High`, `Medium`, or `Low`      |
| `points`       | Effort estimate (integer)                   |
| `dependencies` | List of IDs this item depends on            |
| `status`       | `draft`, `in-progress`, or `done` (customizable, see below) |
| `phase`        | Grouping label for phased delivery          |

## Mixing items with other markdown

yat scans all `.md` files in the items directory. Files without frontmatter or without an `id` field are silently skipped, so plain markdown notes can coexist alongside items.

To explicitly tell yat to skip a file that has frontmatter, add a `yat` directive:

```markdown
---
title: "Sprint retro notes"
yat: ignore
---

These notes won't be loaded by yat.
```

The `yat` field accepts a comma-delimited list of directives. Currently supported:

| Directive | Effect                          |
|-----------|---------------------------------|
| `ignore`  | File is silently skipped by yat |

## Validation

yat catches common mistakes at load time:

- **Duplicate IDs** — two files with the same `id` field is an error.
- **Unknown dependencies** — referencing an ID that doesn't exist in any file.
- **Circular dependencies** — `A → B → A` is detected and reported with the involved IDs.
- **Invalid status/priority** — values not in the recognized set are rejected with the file path.

## Configuration

| Flag / Env            | Default | Description                 |
|-----------------------|---------|-----------------------------|
| `--dir`               | `spec`  | Path to the items directory |
| `YAT_DIR`             | `spec`  | Same, via environment       |
| `-j` / `--json`       | `false` | Output as JSON              |

### Project config file

Instead of passing `--dir` every time, create a `.yat.yaml` (or `.yat.yml`) in your project root — or run `yat init` to generate one:

```yaml
dir: issues
readonly: true  # prevent start/complete from modifying files (useful for CI, shared boards, or AI agents that shouldn't mutate)
```

yat searches from the current directory upward, so the config works from any subdirectory. The `dir` field supports `~` for the home directory.

For per-machine overrides that shouldn't be committed, use `.yat.local.yaml` (or `.yat.local.yml`) — these take priority over the shared config.

**Resolution order:** `--dir` / `YAT_DIR` → `.yat.local.yaml` → `.yat.local.yml` → `.yat.yaml` → `.yat.yml` → `spec`

### Custom statuses

By default, yat recognizes these status values grouped into three categories:

| Group     | Default values                          |
|-----------|-----------------------------------------|
| `done`    | `done`, `complete`, `completed`, `closed` |
| `active`  | `in-progress`, `active`, `started`      |
| `initial` | `draft`, `todo`, `backlog`, `new`       |

Override them in `.yat.yaml` to fit your workflow:

```yaml
statuses:
  done: [done, shipped]
  active: [wip]
  initial: [idea, ready]
```

The first entry in each group is the default for transitions (`yat start` uses the first `active` status, `yat complete` uses the first `done` status).

### Field aliases

If your frontmatter uses different field names, map them to the canonical names:

```yaml
field_aliases:
  status: state
  priority: severity
```

This tells yat to read the `state` field as `status` and `severity` as `priority`.

## JSON output

All commands support `--json` for machine-readable output, useful for scripting or piping into other tools:

```sh
yat status --json | jq '.ready_now'
yat graph --json | jq '.layers'
```

## AI agents

`yat init` detects `CLAUDE.md` and `AGENTS.md` in your project root and offers to append yat usage instructions to them.

Run `yat agents` to print a standalone usage guide that can be pasted into any agent's system prompt.

## Development

```sh
make test      # run tests with race detection
make lint      # golangci-lint
make coverage  # open HTML coverage report
```

## Licence

MIT
