<p align="center">
  <img src="website/public/lasso-logo.png" alt="Lasso logo" width="132" height="132" />
</p>

<h1 align="center">Lasso</h1>

<p align="center">
  <strong>An extensible agent-native monorepo for multi-agent, multi-repo development.</strong><br/>
  One workspace for repositories, worktrees, plugins, and shared project constraints.
</p>

<p align="center">
  <a href="https://dravengarden.github.io/lasso/"><img src="https://img.shields.io/badge/docs-dravengarden.github.io%2Flasso-7c3aed?style=flat-square" alt="Docs" /></a>
  <a href="https://github.com/dravengarden/lasso/actions/workflows/pages.yml"><img src="https://img.shields.io/github/actions/workflow/status/dravengarden/lasso/pages.yml?style=flat-square&label=docs" alt="Docs CI" /></a>
  <a href="https://github.com/dravengarden/lasso/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-Apache%202.0-blue?style=flat-square" alt="License" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/go-1.25+-f5bd27?style=flat-square&logo=go&logoColor=17122d" alt="Go" /></a>
</p>

<p align="center">
  <a href="https://dravengarden.github.io/lasso/guides/quick-start/">Quick start</a> ·
  <a href="https://dravengarden.github.io/lasso/concepts/core-mechanism/">How it works</a> ·
  <a href="https://dravengarden.github.io/lasso/concepts/extensibility/">Extensions</a> ·
  <a href="#install">Install</a>
</p>

---

## What is Lasso?

Lasso is a **monorepo for agent-native development**. It gives an agent one
workspace in which to discover repositories, create isolated worktrees, load
shared skills, and preserve the small amount of coordination that must survive
an individual task.

The repositories remain independent. Lasso does not replace their build
systems, deployment commands, secrets, or runtime state.

```text
many repositories + one agent workspace = one navigable working surface
```

## The core mechanism

Lasso is deliberately built from a small control plane. The flow is:

```text
project-defs/<name>/project.toml
        │  identity only
        ▼
lasso setup
        │  ordinary stable checkout
        ▼
projects/<name>/
        │  native Git worktree operations
        ▼
$HOME/worktrees/<project>/<task>
        │  agent discovers project rules and skills
        ▼
agent runtime executes the work
```

### 1. Declare project identity

Each registered project is one small TOML file:

```toml
kind = "external"
repo = "git@github.com:you/app.git"
default_branch = "main"
```

The registry answers **what the repository is** and **where its stable checkout
belongs**. It intentionally does not contain build, deploy, service-health, or
credential configuration.

### 2. Materialize a workspace

`lasso init` creates the workspace contract:

| File or directory | Role |
|---|---|
| `lasso.toml` | workspace name and selected agent runtimes |
| `lasso.lock.toml` | pinned Lasso core and module versions |
| `project-defs/` | identity-only project registry |
| `projects/` | ordinary stable checkouts |
| `plugins/` | materialized Lasso plugin modules |
| `work-items/` | durable facts for exceptional handoffs |
| `AGENTS.md` / `CLAUDE.md` | runtime discovery and routing |

`lasso setup` then bootstraps registered external repositories into
`projects/<name>/`. A stable checkout is an integration and discovery input;
task work happens in an isolated worktree.

### 3. Give agents an isolated surface

`lasso worktree create <name> --project=<project>` creates a canonical task
worktree under `$HOME/worktrees/<project>/<name>` by default. The command owns
placement, inventory, and conservative garbage collection. Git remains the
source of truth; Lasso does not turn a worktree into a task database.

The active agent runtime still owns the live goal, plan, review, session, and
memory. Lasso supplies the workspace around that execution kernel.

## Ownership boundaries

Clear ownership is the feature that keeps the workspace composable:

| Layer | Lasso provides | Lasso deliberately leaves elsewhere |
|---|---|---|
| Agent runtime | discovery surfaces and plugins | current goal, plan, session, memory, review |
| Lasso workspace | project identity, stable checkouts, task-worktree convention | project build graph and task runner |
| Durable coordination | minimal work items for facts and handoffs | an agent execution state machine |
| Owning project | — | build, test, deploy, secrets, service health |
| Optional modules | versioned conventions, checks, skills, hooks | one giant forked core |

That boundary lets Lasso work with a Go service, a Rust library, a Nix host,
or a true subdirectory project without asking them to share a toolchain.

## The extension model

Lasso has an allowlisted core and an installable module catalog. A module is a
versioned capability pack, not a patch to the CLI:

```text
modules/catalog.toml
        │
        │  lasso module add <id>
        ▼
.lasso/modules/<id>/  +  lasso.lock.toml
        │
        ├── conventions/       merge project conventions
        ├── plugin/             materialize skills, hooks, and manifests
        └── scripts/            ship deterministic helpers
```

Modules can be three things:

| Kind | Use it for | Example |
|---|---|---|
| `convention` | language or repository guidance | `lang-rust`, `lang-nix` |
| `capability` | deterministic checks and workspace contracts | `docs-index` |
| `plugin` | skills, hooks, and runtime-facing workflows | `security`, `memory` |

Installing a module:

1. Reads its entry from `modules/catalog.toml`.
2. Copies the payload to `.lasso/modules/<id>/`.
3. Pins the version in `lasso.lock.toml`.
4. Merges conventions when present.
5. Materializes `plugins/lasso-<id>/` when it contains a plugin.
6. Refreshes Codex and Claude-compatible marketplaces.

An extension author adds a directory and a catalog entry; the core CLI does not
need a new command for every new capability:

```text
modules/<id>/
├── module.toml
├── conventions/       optional guidance
├── plugin/            optional skills + runtime manifests
└── scripts/           optional deterministic helpers
```

The canonical skill implementation is shared. Runtime-specific manifests are
thin discovery adapters, so adding a runtime does not require forking every
skill.

## Install

```bash
git clone git@github.com:dravengarden/lasso.git
cd lasso
just build
export PATH="$PWD/bin:$PATH"
export LASSO_KIT_ROOT="$PWD"
```

Requires Go 1.25+ (or `nix develop`).

## Quick start

```bash
# Create a workspace and choose the runtimes/modules it should expose.
lasso init ~/my-fleet \
  --name my-fleet \
  --runtime=codex,claude \
  --module=lang-go

cd ~/my-fleet

# Register a repository and create its stable checkout.
lasso project add --project=app --repo-url=git@github.com:you/app.git
lasso setup --only=app

# Create an isolated task surface.
lasso worktree create feature-login --project=app
```

Install the core plugin through the workspace marketplace:

```bash
codex plugin marketplace add .
codex plugin add lasso-core@lasso

# Claude / Grok-compatible followers
claude plugin marketplace add .
claude plugin install lasso-core@lasso
```

Add or remove capabilities without forking Lasso:

```bash
lasso module list
lasso module add security
lasso module add lang-rust
lasso module add npm-release
lasso module add pypi-release
lasso module add cargo-release
lasso module remove security --yes
```

## Lasso is not a build system

| | Build systems | Lasso |
|---|---|---|
| Primary object | packages and build graph | repositories and agent contracts |
| Execution kernel | own task runner | the native agent runtime |
| Cross-repo workflow | generators / project graph | registry, checkouts, worktrees, skills |
| Owns builds and deploys | yes | no |
| Extension unit | plugin or generator | versioned module |

Use Lasso above Bazel, Buck, Nx, Moon, Make, Cargo, or the project’s own
toolchain. It makes those systems easier for an agent to find; it does not
compete with them.

## Repository layout

```text
cmd/lasso/              CLI entrypoint
internal/               registry, project, worktree, module, work-item logic
plugins/lasso-core/     installable core skills
modules/                optional capability packs + catalog.toml
templates/workspace/    generated workspace contract
website/                Astro Starlight documentation site
docs/                   product contracts and design notes
```

## Development

```bash
just build
just test
just verify          # format, vet, tests, skills, docs index
just website-build   # Astro production build
```

Read the [core mechanism](https://dravengarden.github.io/lasso/concepts/core-mechanism/)
and [extension model](https://dravengarden.github.io/lasso/concepts/extensibility/)
for the detailed contract.

## Status

v0.1 — usable for scaffolding workspaces and iterating on the module/skill
model. APIs may still shift.

## License

[Apache License 2.0](LICENSE)
