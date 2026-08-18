---
title: Concepts overview
description: What Lasso is — and what it deliberately is not.
---

## Product category

Lasso is an **agent-native monorepo workspace** tool.

It is closer in *category* to Bazel / Nx (multi-project workspace tooling) than
to an application scaffold — but it is **not** a build system.

```text
Nx / Bazel / Moon     →  how packages build & test
Lasso                 →  how agents discover projects, isolate worktrees,
                         share conventions, coordinate durably, and verify
```

A workspace may contain:

- **external** projects (ordinary clones)
- **subdir** projects (true monorepo trees)

## Ownership split

| Concern | Owner |
|---|---|
| Goal, plan, review, session, memory | Agent runtime |
| Project identity & stable checkouts | Lasso |
| Worktree placement / GC | Lasso (state-free) |
| Durable cross-session facts | Lasso work items |
| Build / test / deploy / secrets | Owning project |
| Optional capabilities | Modules |

## Instance vs product

| | |
|---|---|
| **Lasso** | The product (this repository) |
| **Your fleet** | A workspace created by `lasso init` (e.g. a personal Columbus-like instance) |

Change harness capability → change **lasso**.  
Change your repos / machines / business docs → change **your workspace**.
