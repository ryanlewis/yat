# yat (issue tracker)

yat is a CLI issue tracker. Items (stories, tasks, spikes) are Markdown files with YAML frontmatter, kept in an items directory (`spec/` by default, or the `dir` set in `.yat.yaml`). yat builds a dependency graph from them and tells you what to work on next.

## Safety

- **Safe to run freely**: `yat status`, `yat next`, `yat ready`, `yat show <id>`, `yat blocked`, `yat list`, `yat graph` only read item files. `yat skill list` and `yat skill show` write nothing.
- **Writes change item files in place**: `yat start <id>` and `yat complete <id>` rewrite the item's `status` field and keep the rest of the file as it was.
- **Read-only projects**: when `.yat.yaml` has `readonly: true`, `start` and `complete` fail with `project is read-only (readonly: true in .yat.yaml)`. This is expected, not a fault. Do the work the item describes and skip the status update. Do not edit the config or the item's frontmatter to get around it.
- `yat init` creates `.yat.yaml`, an items directory and an example item, and can append a short pointer to this skill to `CLAUDE.md` / `AGENTS.md`. Run it only when asked.
- `yat skill install` / `yat skill uninstall` write to the agent's config directory. Do not run them unasked.

## Workflow

1. `yat status` — overview: counts by status and type, what is ready now.
2. `yat next` — the single highest-priority ready item, with its full body.
3. `yat start <id>` — mark it in progress.
4. Do the work the item body describes. `yat show <id>` re-reads it.
5. `yat complete <id>` — mark it done. Read the "Now unblocked" part of the output.
6. Repeat from step 2.

Let `yat next` choose rather than picking items yourself. It respects priority (Critical > High > Medium > Low) and dependency order. An item is ready only when every item in its `dependencies` is done; `yat start` refuses a blocked item with `cannot start <id>: blocked by <ids>`.

## Commands

| Command | What it does |
| --- | --- |
| `yat status` | Overview of all items: counts, points, ready set, deepest layer. |
| `yat next` | The highest-priority ready item, including its body. |
| `yat ready` | Every item ready to work on. `-a` / `--all` also lists in-progress items. |
| `yat show <id>` | Full details and body of one item. |
| `yat start <id>` | Mark an item in progress. Writes. |
| `yat complete <id>` | Mark an item done and report what it unblocked. Writes. |
| `yat blocked` | Items that cannot start yet, and what each waits on. |
| `yat list` | All items grouped by status. `-p` / `--phase <name>` filters; `-p active` is the current phase. |
| `yat graph` | The dependency graph as topological layers. |
| `yat init [<dir>]` | Set up a new project in the current directory. |

Global flags: `--dir <path>` (or `$YAT_DIR`) points at the items directory and overrides `.yat.yaml`; `-j` / `--json` switches output to JSON.

## Output and `--json`

Always pass `--json` when you parse output. The shapes you will use most:

- `yat status --json` — `{"total_items", "total_points", "by_type", "by_status", "active_phase", "by_phase", "ready_now": [ids], "deepest_layer", "deepest_items"}`.
- `yat next --json` — `{"id", "title", "priority", "type", "points", "phase", "file_path", "body"}`, or `null` when nothing is ready.
- `yat ready --json` — `[{"id", "priority", "type", "status", "title"}]`.
- `yat show <id> --json` — the item, including `dependencies`, `status` and `body`.
- `yat complete <id> --json` — `{"id", "title", "unblocked": [{"id", "title"}], "still_blocked": [{"id", "title", "waiting_on"}]}`.
- `yat blocked --json` — `[{"id", "title", "waiting_on": [ids]}]`.
- `yat graph --json` — `{"layers": [[ids]], "edges": [{"from", "to"}]}`.

Errors are not JSON: they print `error: <message>` to stderr and exit 1, with or without `--json`. Branch on the exit status.

## Common flows

Pick up the next item and read it:

```sh
id=$(yat next --json | jq -r '.id // empty')
[ -n "$id" ] && yat show "$id" --json | jq -r '.body'
```

Work it end to end:

```sh
yat start "$id"
# ... do the work ...
yat complete "$id" --json | jq -r '.unblocked[].id'
```

See what is ready and what is stuck:

```sh
yat status --json | jq '.ready_now'
yat blocked --json | jq -r '.[] | "\(.id) waits on \(.waiting_on | join(", "))"'
```

## Errors to expect

- `items directory "spec" not found.` (followed by a hint to run `yat init`) — there is no items directory where yat looked. Run `yat init` to create a project (only if asked), or pass `--dir <path>` to point at an existing one.
- `project is read-only (readonly: true in .yat.yaml)` — see Safety: do the work, skip the status update.
- `item "<id>" not found` — check the ID with `yat list --json`. IDs are matched exactly.
- `cannot start <id>: blocked by <ids>` — finish the listed items first, or ask which to do.
- `item <id> is already done` — nothing to do; move on with `yat next`.
- `cannot update status in <file>: its value is not a one-line scalar; edit it by hand` — the status is written over several lines or uses a YAML anchor, so yat left the file untouched. Tell the user; do not rewrite the frontmatter yourself unless asked.
