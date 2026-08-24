---
title: Extension model
description: Add conventions, checks, skills, hooks, and scripts without forking Lasso core.
---

## Modules are the extension unit

Lasso keeps its core allowlisted and moves optional behavior into versioned
modules. The kit catalog is the source of available capability packs:

```text
modules/catalog.toml
        │
        │  lasso module add <id>
        ▼
.lasso/modules/<id>/  +  lasso.lock.toml
        │
        ├── conventions/       merge guidance into the workspace
        ├── plugin/             materialize skills, hooks, manifests
        └── scripts/            deterministic helpers
```

Installing a module is an explicit, reversible workspace change. The version
is pinned in `lasso.lock.toml`; removing the module removes its materialized
payload and refreshes the runtime marketplaces.

## The module contract

Each catalog entry identifies a payload and its behavior:

```toml
[[module]]
id = "security"
version = "0.2.0"
kind = "plugin"
description = "Deterministic dependency and supply-chain scanning"
path = "modules/security"
skills = ["security-scan"]
requires = []
default = false
```

The supported kinds are intentionally small:

| Kind | Adds | Example |
|---|---|---|
| `convention` | language or repository guidance | `lang-go`, `lang-rust` |
| `capability` | deterministic checks and contracts | `docs-index` |
| `plugin` | skills, hooks, and runtime-facing workflows | `security`, `memory` |

## What `module add` does

```bash
lasso module list
lasso module add security
lasso module list --installed
lasso module remove security --yes
```

The install path is deterministic:

1. Read and validate the module from `modules/catalog.toml`.
2. Check declared module dependencies.
3. Copy the module payload to `.lasso/modules/<id>/`.
4. Pin `id → version` in `lasso.lock.toml`.
5. Merge `conventions/` when the module supplies it.
6. Materialize `plugins/lasso-<id>/` when `plugin/` exists.
7. Refresh Codex and Claude-compatible marketplace manifests.

The CLI owns this deterministic state transition. Skills provide the human and
agent-facing judgment around when and why to install a module.

## Author a module

The smallest useful module is a directory plus one catalog entry:

```text
modules/<id>/
├── module.toml
├── conventions/       optional guidance
├── plugin/             optional runtime-facing package
│   ├── .codex-plugin/
│   ├── .claude-plugin/
│   ├── skills/
│   ├── hooks/          optional
│   └── scripts/        optional
└── scripts/            optional kit-side helpers
```

Use a `convention` when the module only teaches project rules. Use a
`capability` when it adds deterministic validation. Use a `plugin` when an
agent needs an invokable skill, hook, or runtime integration.

## One canonical skill tree

Codex is the design baseline. Other runtimes follow through shared skill bodies
and thin discovery adapters:

```text
one canonical SKILL.md
        ├── .codex-plugin/plugin.json
        └── .claude-plugin/plugin.json
```

Do not fork a skill once per runtime. If a runtime needs different discovery,
add the smallest manifest or adapter at the edge and keep the behavior shared.

## Extension boundaries

Use the narrowest surface that solves the problem:

| Need | Extension surface |
|---|---|
| tell agents how to work in a project | project `AGENTS.md` / conventions module |
| add a repeatable agent workflow | plugin skill |
| block an unsafe tool action | plugin hook |
| add a deterministic check | capability module script |
| add a language pack | `lang-*` convention module |
| change repository identity | project registry entry |
| change build/deploy behavior | owning project or host configuration |

If a capability has no residual workspace, host, cross-project, or external
system gap, it does not belong in Lasso core. Start with a module; promote
behavior into core only when the product contract proves the gap is durable.
