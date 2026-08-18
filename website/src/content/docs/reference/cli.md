---
title: CLI reference
description: Overview of lasso subcommands.
---

```bash
lasso --help
lasso --root <path> …          # override workspace root
```

## Workspace lifecycle

| Command | Purpose |
|---|---|
| `lasso init` | Scaffold a new workspace |
| `lasso setup` | Clone missing external projects |
| `lasso module list` | List kit or installed modules |
| `lasso module add` | Install a module |
| `lasso module remove` | Remove a module (`--yes`) |

## Projects

| Command | Purpose |
|---|---|
| `lasso project list` | List registry |
| `lasso project path` | Resolve stable checkout |
| `lasso project add` | Register |
| `lasso project update` | Reconcile / fast-forward |
| `lasso project rm` | Unregister |

## Worktrees

| Command | Purpose |
|---|---|
| `lasso worktree create` | Canonical placement |
| `lasso worktree list` | Inventory |
| `lasso worktree remove` | Remove one |
| `lasso worktree gc` | GC clean merged aged trees |

## Work items

| Command | Purpose |
|---|---|
| `lasso work-item new` | Create metadata |
| `lasso work-item list` | List / `--current` |
| `lasso work-item show` | Inspect |
| `lasso work-item validate` | Schema check |
| `lasso work-item rm` | Delete (`--yes`) |

Prefer `--format=json` for agent parsing. Subcommand `--help` is authoritative.
