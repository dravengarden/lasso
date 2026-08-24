---
title: Core mechanism
description: How Lasso turns repository identity into an agent-ready workspace.
---

## The lifecycle

Lasso has one small control plane and a predictable lifecycle:

```text
project-defs/<name>/project.toml
        │  identity only
        ▼
lasso setup
        │  ordinary stable checkout
        ▼
projects/<name>/
        │  canonical Git worktree
        ▼
$HOME/worktrees/<project>/<task>
        │  project rules + agent skills
        ▼
native agent runtime executes the task
```

The point is not to hide Git or the project’s toolchain. The point is to give
an agent a stable map of the workspace and a safe place to work.

## 1. Initialize the contract

`lasso init` renders a workspace from `templates/workspace/` and writes the
small mutable files that describe the instance:

| Artefact | Meaning |
|---|---|
| `lasso.toml` | workspace name and selected runtimes |
| `lasso.lock.toml` | Lasso core and module version pins |
| `AGENTS.md` | Codex-first routing and workspace boundaries |
| `CLAUDE.md` | follower entrypoint when Claude/Grok is selected |
| `.agents/plugins/marketplace.json` | Codex marketplace entries |
| `.claude-plugin/marketplace.json` | Claude-compatible marketplace entries |
| `project-defs/registry.toml` | registry format marker |
| `work-items/` | durable coordination template |

Initialization also installs `lasso-core` and any default modules from the kit
catalog.

## 2. Register identity, not operations

Every project is represented by `project-defs/<name>/project.toml`:

```toml
kind = "external"
repo = "git@github.com:you/app.git"
default_branch = "main"
```

The registry supports two project kinds:

| Kind | Checkout model |
|---|---|
| `external` | ordinary Git clone under `projects/<name>/` |
| `subdir` | project already lives inside the workspace repository |

The registry intentionally excludes build commands, deployment DSLs, service
health, credentials, and application catalogs. Those belong to the project or
host that owns them.

## 3. Bootstrap a stable checkout

```bash
lasso project add --project=app --repo-url=git@github.com:you/app.git
lasso setup --only=app
lasso project path app
```

`lasso setup` creates ordinary stable clones for external projects. They are
discovery and integration inputs, not task scratch space. Once a checkout is
resolved, the agent follows the nearest project-owned `AGENTS.md` and commands.

## 4. Isolate task work

```bash
lasso worktree create feature-login --project=app
lasso worktree list --project=app --format=json
lasso worktree gc --project=app
```

The canonical root is `$HOME/worktrees` by default and can be changed with
`LASSO_WORKTREE_ROOT`. A task worktree is placed at:

```text
$HOME/worktrees/<project>/<name>
```

Lasso owns placement, inventory, and conservative garbage collection. Git owns
the branch and worktree state. The agent runtime owns the live task state.

## 5. Preserve only exceptional coordination

Work items are for facts that must outlive the current agent task: a durable
handoff, an external blocker, or an auditable decision. They are not a second
execution state machine.

```bash
lasso work-item new \
  --id=release-cutover \
  --title="Coordinate the release cutover" \
  --project=app \
  --recipe=ops
```

Goals, plans, progress, reviewers, conversation, and active memory remain in
the native runtime.

## The boundary

```text
agent runtime  ── live execution: goal, plan, review, session, memory
       │
       ▼
Lasso           ── workspace map: registry, checkouts, worktrees, modules
       │
       ▼
project/host    ── owned operations: build, test, deploy, secrets, health
```

This separation is the core mechanism. It lets Lasso coordinate many project
types without taking ownership of their toolchains.
