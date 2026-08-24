---
title: Concepts overview
description: The short version of Lasso’s product model and ownership boundary.
---

## The short version

Lasso is an **agent-native monorepo**: one workspace where agents can discover
repositories, create isolated worktrees, load shared skills, and preserve
durable handoffs.

The monorepo is a workspace and coordination surface, not one forced source
tree. External repositories remain independent clones; subdirectory projects
can remain true monorepo trees.

```text
project identity → stable checkout → task worktree → agent execution
```

## Read next

- [Core mechanism](/lasso/concepts/core-mechanism/) — the lifecycle from
  `lasso init` to an isolated task surface.
- [Extension model](/lasso/concepts/extensibility/) — how modules add
  conventions, checks, skills, hooks, and scripts.
- [Agent runtimes](/lasso/concepts/agent-runtimes/) — the Codex baseline and
  thin follower adapters.

## What belongs where

| Concern | Owner |
|---|---|
| Current goal, plan, review, session, memory | Agent runtime |
| Project identity and stable checkout bootstrap | Lasso |
| Canonical worktree placement and safe GC | Lasso, with Git as source of truth |
| Durable facts and exceptional handoffs | Lasso work items |
| Build, test, deploy, secrets, service health | Owning project or host |
| Optional conventions, checks, skills, and hooks | Versioned modules |

Lasso stays useful by knowing where to stop. It creates a navigable workspace
around project toolchains; it does not create a second build system or agent
task runtime.

## Product vs generated workspace

| | |
|---|---|
| **Lasso** | The product (this repository) |
| **Your workspace** | A workspace created by `lasso init` |

Change a workspace capability → change **Lasso** or add a module.

Change your repositories, machines, or business docs → change **your workspace**.
