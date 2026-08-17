# Lasso

**Lasso** is an agent-native monorepo workspace tool.

It ropes repositories into one workspace for AI agents: project registry, stable
checkouts, state-free worktrees, durable work items, shared conventions, and
installable modules — without owning agent task state or each project's build
system.

| Concern | Owner |
|---|---|
| Goal, plan, review, session, memory | Agent runtime (Codex kernel; Claude/Grok followers) |
| Project identity & stable checkouts | Lasso |
| Build, test, deploy | Owning project |
| Optional capabilities | Lasso modules |

## Quick start

```bash
# In this product repository
nix develop   # or ensure Go 1.25+, just, git
just build
export PATH="$PWD/bin:$PATH"
export LASSO_KIT_ROOT="$PWD"

# Create a workspace
lasso init ~/my-fleet --name my-fleet --runtime=codex,claude --module=lang-go
cd ~/my-fleet

# Install the core plugin into your agent runtime
codex plugin marketplace add .
codex plugin add lasso-core@lasso
```

## Modules (incremental install / remove)

```bash
lasso module list                 # from kit catalog
lasso module list --installed     # in current workspace
lasso module add security
lasso module add lang-rust
lasso module remove security --yes
```

Guided agent workflows:

| Skill | Purpose |
|---|---|
| `lasso-init` | Scaffold a workspace |
| `lasso-add-module` | Install a module |
| `lasso-remove-module` | Remove a module |
| `lasso-add-language` | Install `lang-*` convention packs |
| `project-add` / `project-rm` / `project-update` | Registry lifecycle |
| `work-item` | Durable cross-session coordination |
| `verify-change` | Deterministic gates + proportional review |
| `lasso-doctor` | Residual workspace discovery checks |

## Layout (product / kit)

```text
cmd/lasso/                 CLI
internal/                  registry, modules, worktree, work-items
plugins/lasso-core/        installable core skills
modules/                   optional capability packs + catalog.toml
templates/workspace/       init template
skills/                    kit-local copies of meta-skills
docs/contracts/            product contracts
```

## Layout (generated workspace)

```text
lasso.toml                 name + runtimes
lasso.lock.toml            pinned modules
project-defs/              identity-only registry
projects/                  stable checkouts
work-items/                durable coordination
plugins/lasso-core/        vendored core plugin
.lasso/modules/            installed module payloads
AGENTS.md / CLAUDE.md      agent guidance
```

## Design principles

1. Codex is the execution kernel; Claude and Grok follow via shared artifacts and thin adapters.
2. Core is an allowlist: identity, checkouts, worktrees, work-items, module install.
3. Skills encode judgment; the CLI encodes deterministic state changes.
4. Modules evolve independently of the Go core.

See [docs/contracts/requirements.md](docs/contracts/requirements.md).
