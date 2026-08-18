<p align="center">
  <img src="website/public/logo.svg" alt="Lasso" width="96" height="96" />
</p>

<h1 align="center">Lasso</h1>

<p align="center">
  <strong>Agent-native monorepo workspace</strong><br/>
  Rope repositories into one workspace for AI agents — without owning their build systems or task state.
</p>

<p align="center">
  <a href="https://dravengarden.github.io/lasso/"><img src="https://img.shields.io/badge/docs-dravengarden.github.io%2Flasso-0ea5e9?style=flat-square" alt="Docs" /></a>
  <a href="https://github.com/dravengarden/lasso/actions/workflows/pages.yml"><img src="https://img.shields.io/github/actions/workflow/status/dravengarden/lasso/pages.yml?style=flat-square&label=docs" alt="Docs CI" /></a>
  <a href="https://github.com/dravengarden/lasso/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue?style=flat-square" alt="License" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.25+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go" /></a>
</p>

<p align="center">
  <a href="https://dravengarden.github.io/lasso/">Documentation</a> ·
  <a href="https://dravengarden.github.io/lasso/guides/quick-start/">Quick start</a> ·
  <a href="https://dravengarden.github.io/lasso/concepts/modules/">Modules</a> ·
  <a href="#install">Install</a>
</p>

---

## Why Lasso?

Bazel, Buck, Nx, and Moon own **build graphs and task runners**.  
Lasso owns the layer **above** that for the agent era:

| Concern | Owner |
|---|---|
| Goal, plan, review, session, memory | Agent runtime (Codex kernel; Claude / Grok followers) |
| Project identity, stable checkouts, worktrees | **Lasso** |
| Durable cross-session coordination | **Lasso work items** |
| Build, test, deploy, secrets | Owning project |
| Optional capabilities | **Lasso modules** (add / remove) |

One sentence:

> **Lasso ropes repos into an agent-ready monorepo workspace — registry, conventions, plugins, and verification — while each project keeps its toolchain and each agent keeps its execution state.**

## Features

- **Project registry** — identity-only TOML (`kind`, `repo`, `default_branch`); no deploy DSL in core
- **Stable checkouts + worktrees** — bootstrap ordinary clones; state-free worktree placement / GC
- **Installable modules** — `lasso module add|remove` with lockfile pins and dual agent marketplaces
- **Skills-first UX** — `lasso-init`, `lasso-add-module`, `work-item`, `verify-change`, …
- **Multi-runtime** — Codex baseline; Claude Code & Grok Build via shared skills + thin adapters
- **Thin core** — optional packs (docs-index, lang-*, security, …) evolve independently

## Install

```bash
git clone git@github.com:dravengarden/lasso.git
cd lasso
just build                 # → ./bin/lasso
export PATH="$PWD/bin:$PATH"
export LASSO_KIT_ROOT="$PWD"
```

Requires Go 1.25+ (or `nix develop`).

## Quick start

```bash
# 1. Create a workspace
lasso init ~/my-fleet \
  --name my-fleet \
  --runtime=codex,claude \
  --module=lang-go

cd ~/my-fleet

# 2. Install the core plugin into your agent runtime
codex plugin marketplace add .
codex plugin add lasso-core@lasso

# Claude / Grok followers
claude plugin marketplace add .
claude plugin install lasso-core@lasso

# 3. Register a repository
lasso project add --project=app --repo-url=git@github.com:you/app.git
lasso setup --only=app
```

### Modules

```bash
lasso module list                 # kit catalog
lasso module list --installed     # this workspace
lasso module add security
lasso module add lang-rust
lasso module remove security --yes
```

Guided agent skills: `lasso-init` · `lasso-add-module` · `lasso-remove-module` · `lasso-add-language`

## How it compares

| | Bazel / Buck2 | Nx / Moon | **Lasso** |
|---|---|---|---|
| Primary object | Build graph | Project / task graph | **Repo graph + agent contracts** |
| Execution kernel | Own runner | Own task runner | **Agent runtime** |
| Cross-repo agent workflows | — | Generators / plugins | **Skills + plugins** |
| Owns project builds | Yes | Often | **No** |

## Documentation

Full docs (Astro Starlight): **[dravengarden.github.io/lasso](https://dravengarden.github.io/lasso/)**

| Topic | Link |
|---|---|
| Quick start | [guides/quick-start](https://dravengarden.github.io/lasso/guides/quick-start/) |
| Concepts | [concepts/overview](https://dravengarden.github.io/lasso/concepts/overview/) |
| Modules | [concepts/modules](https://dravengarden.github.io/lasso/concepts/modules/) |
| Agent runtimes | [concepts/agent-runtimes](https://dravengarden.github.io/lasso/concepts/agent-runtimes/) |
| Requirements | [reference/requirements](https://dravengarden.github.io/lasso/reference/requirements/) |

Local preview:

```bash
cd website && npm install && npm run dev
```

## Project layout

**Product (this repo)**

```text
cmd/lasso/              CLI
internal/               registry, modules, worktree, work-items
plugins/lasso-core/     installable core skills
modules/                optional capability packs + catalog.toml
templates/workspace/    init template
website/                Astro Starlight docs site
docs/                   source contracts (also published via website)
```

**Generated workspace**

```text
lasso.toml / lasso.lock.toml
project-defs/   projects/   work-items/
plugins/lasso-core/   .lasso/modules/
AGENTS.md / CLAUDE.md
```

## Development

```bash
just build
just test
just verify          # format, vet, tests, skills, docs index
just website-build   # Astro production build
```

## Design principles

1. **Codex is the execution kernel** — Claude and Grok follow via shared artifacts and thin adapters.
2. **Core is an allowlist** — identity, checkouts, worktrees, work-items, module install.
3. **Skills encode judgment; the CLI encodes deterministic state** — add/remove modules without forking core.
4. **Modules evolve independently** — pin versions in `lasso.lock.toml`.

See [requirements](https://dravengarden.github.io/lasso/reference/requirements/).

## Status

v0.1 — usable for scaffolding workspaces and iterating on the module/skill model. APIs may still shift.

## License

[Apache License 2.0](LICENSE)
