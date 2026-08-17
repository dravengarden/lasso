---
name: lasso
description: Route and operate the Lasso agent-native monorepo CLI for project lookup, stable checkout resolution, modules, and exceptional durable work items. Use for general lasso questions or workflows combining these areas. For explicit project registration, removal, module install, or reconciliation, use the matching specialized skill instead.
---

# Lasso CLI

Run the `lasso` binary from the user's current checkout. It resolves the
workspace root through `--root`, `LASSO_ROOT`, or cwd discovery. Set
`LASSO_KIT_ROOT` to the Lasso product checkout when adding or listing kit
modules outside that tree.

```bash
lasso --help
```

## Routing

| Need | Command or owner |
|---|---|
| Create a workspace | `lasso-init` skill or `lasso init` |
| Inspect projects | `lasso project list` |
| Register/remove/reconcile projects | `project-add`, `project-rm`, or `project-update` skill |
| Resolve the stable checkout | `lasso project path <name>` |
| Install/remove capability packs | `lasso-add-module` / `lasso-remove-module` or `lasso module` |
| Task worktrees | Native runtime surfaces; `lasso worktree` for canonical placement/GC |
| Durable exceptional coordination | `lasso work-item` plus `work-item` skill |
| Current objective, plan, review | Native active-runtime surfaces; Codex remains the baseline |
| Build, lint, test, run | owning project's AGENTS.md and commands |

Prefer `--format=json` for structured inspection.

## Boundaries

- A work item does not own a branch, checkout, or agent-runtime worktree.
- Stable ordinary checkouts are discovery/integration inputs; prefer isolated
  worktrees for mutations.
- Project TOML contains identity only.
- Do not recreate a second agent task runtime inside Lasso.
