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

Create a directory and add some items:

```
issues/
  stories/
    ST-001-auth-flow.md
  tasks/
    TK-001-setup-ci.md
  spikes/
    SP-001-evaluate-db.md
```

Each file uses YAML frontmatter:

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
yat status        # overview of all items
yat ready         # items with all dependencies met
yat next          # highest-priority ready item
yat start TK-001  # mark as in-progress
yat complete TK-001  # mark as done
yat blocked       # items waiting on dependencies
yat graph         # dependency graph by layer
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
| `status`       | `draft`, `in-progress`, or `done`           |
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

## Configuration

| Flag / Env            | Default | Description                 |
|-----------------------|---------|-----------------------------|
| `--dir`               | `spec`  | Path to the items directory |
| `YAT_DIR`             | `spec`  | Same, via environment       |
| `-j` / `--json`       | `false` | Output as JSON              |

### Project config file

Instead of passing `--dir` every time, you can create a `.yat.yaml` (or `.yat.yml`) in your project root:

```yaml
dir: issues
```

yat searches from the current directory upward, so the config works from any subdirectory. The `dir` field supports `~` for the home directory.

For per-machine overrides that shouldn't be committed, use `.yat.local.yaml` (or `.yat.local.yml`) — these take priority over the shared config.

**Resolution order:** `--dir` / `YAT_DIR` → `.yat.local.yaml` → `.yat.local.yml` → `.yat.yaml` → `.yat.yml` → `spec`

## JSON output

All commands support `--json` for machine-readable output, useful for scripting or piping into other tools:

```sh
yat status --json | jq '.ready_now'
yat graph --json | jq '.layers'
```

## Development

```sh
make test      # run tests with race detection
make lint      # golangci-lint
make coverage  # open HTML coverage report
```

## Licence

MIT
