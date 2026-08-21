# Columbus on Lasso — alignment contract

**Status:** accepted direction for migration  
**Date:** 2026-08-21

Columbus is an **instance workspace**. Lasso is the **business-agnostic product**
it should consume. This note freezes the ownership split and the migration
order so the two repos stop drifting as parallel forks.

## Ownership

| Concern | Owner |
|---|---|
| Project registry, setup, path resolution | **Lasso** |
| State-free worktrees, work-items | **Lasso** |
| Module catalog / lockfile / init template | **Lasso** |
| Core skills (`project-*`, `work-item`, `verify-change`, meta-skills) | **Lasso** |
| Optional hooks, security, memory, lang packs | **Lasso modules** (generic, configurable) |
| Real `project-defs/`, checkouts, active work-items | **Columbus** |
| `machines/`, `resources/`, CUE instance data, activate/release | **Columbus** |
| Host path defaults, hawk/falcon routing, Cowboy/LiveView conventions | **Columbus** |
| Agent task/goal/plan/memory/session state | **Agent runtimes** (never either repo) |

## Product vs instance artifacts

```text
Lasso (github.com/dravengarden/lasso)
  cmd/lasso · modules/ · plugins/lasso-core · templates/

Columbus (instance)
  lasso.toml + lasso.lock.toml     # pin Lasso core + modules
  project-defs/ · projects/ · work-items/
  machines/ · resources/ · schema/ # fleet
  AGENTS.md                        # routes to Lasso + fleet docs
```

## Compatibility bridge (transition)

While Columbus migrates:

| Lasso canonical | Accepted legacy |
|---|---|
| `LASSO_ROOT` | `COLUMBUS_ROOT` |
| `LASSO_WORKTREE_ROOT` | `COLUMBUS_WORKTREE_ROOT` |
| `LASSO_CONFIG` | `COLUMBUS_CONFIG` |
| `LASSO_MACHINE_ID` | `COLUMBUS_MACHINE_ID` |
| `lasso` CLI API 2 | `harness-cli` API 2 (to be replaced) |

Legacy names are **read fallbacks only**. New Columbus docs and services should
emit Lasso names.

Precedence: `--root` > `LASSO_ROOT` > `COLUMBUS_ROOT` > cwd walk-up.  
If `COLUMBUS_ROOT` is set on a host, use `LASSO_ROOT` (or `--root`) when operating
on a different workspace so the legacy env does not pin discovery.

## What must not move into Lasso core

- Personal absolute paths (`/home/draven/...`)
- Named host special-cases (`hawk` / `falcon`) without config
- Live cloud resource inventories
- Business project cards and domain design docs
- UI / PWA / LiveView product conventions

NixOS transaction fences belong in the **hooks** module and activate only when
`machine.nixos_hosts` in config lists the current machine id.

## Migration phases

1. **Lasso hygiene** — remove Columbus residue; dual-read legacy env; ship
   generic `hooks` module *(this change)*.
2. **Columbus consume** — add `lasso.toml` / lock; install modules from
   `LASSO_KIT_ROOT`; prefer `lasso` over `harness-cli` (shim allowed).
3. **Deprecate fork** — delete duplicated Go CLI / core plugin from Columbus
   once gates pass on Lasso binaries.
4. **Lift optional depth** — promote Columbus security/memory implementations
   into Lasso modules (configurable; no DeepSeek/hawk assumptions in defaults).
5. **Verify split** — Lasso `just verify` vs Columbus fleet checks
   (`machines-*`, `resources-*`, …).

## Acceptance for “aligned”

- Columbus documents “based on Lasso” and pins a Lasso revision/lock.
- Stable checkout / work-item / module operations run through `lasso` API 2.
- No required `/home/draven/columbus` default in Lasso product code.
- Fleet data remains only in Columbus.

## Non-goals

- Rewriting Columbus NixOS in this phase
- Making Lasso a build system (Nx/Bazel replacement)
- Syncing agent transcripts across runtimes
