---
title: Install
description: Install the Lasso CLI from source.
---

## From source (recommended while v0.1)

```bash
git clone git@github.com:dravengarden/lasso.git
cd lasso
just build          # writes ./bin/lasso
```

Put `./bin` on your `PATH`, or run `./bin/lasso` directly.

### Toolchain

| Tool | Notes |
|---|---|
| Go 1.25+ | Required to build the CLI |
| Git | Required for checkouts and worktrees |
| Nix (optional) | `nix develop` for a pinned shell |

```bash
nix develop   # optional
just doctor
```

## Environment variables

| Variable | Purpose |
|---|---|
| `LASSO_ROOT` | Workspace root override (normally discovered via `project-defs/registry.toml`) |
| `LASSO_KIT_ROOT` | Product checkout that ships `modules/catalog.toml` |
| `LASSO_WORKTREE_ROOT` | Canonical worktree parent (default `$HOME/worktrees`) |

When developing Lasso itself:

```bash
export LASSO_KIT_ROOT=/path/to/lasso
```

## Verify

```bash
lasso version --format=json
lasso module list --format=json
```
