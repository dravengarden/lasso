# Module system

## Units and boundaries

Lasso has three nested extension units:

| Unit | Owner | Meaning |
|---|---|---|
| module | Lasso workspace | smallest catalog install, pin, dependency, and remove unit |
| plugin | agent runtime | materialized discovery and distribution boundary for skills, scripts, assets, hooks, MCP, or apps |
| skill | agent | smallest triggered workflow and tool-usage contract inside a plugin |

A plugin module currently materializes exactly one `plugins/lasso-<module>/`
tree. Split capabilities into separate modules when users should install,
upgrade, or remove them independently. Keep multiple related skills in one
plugin when they share that installation boundary; do not create a plugin per
command.

## Catalog

The product repository ships `modules/catalog.toml`. Each entry has:

| Field | Meaning |
|---|---|
| `id` | stable install id (`lang-go`, `security`) |
| `version` | semver-ish pin written to the workspace lockfile |
| `kind` | `plugin`, `convention`, or `capability` |
| `path` | path under the kit |
| `requires` | other module ids that must be installed first |
| `default` | installed automatically by `lasso init` |

## Workspace install layout

```text
.lasso/modules/<id>/     copied module payload
lasso.lock.toml          pins id → version
plugins/lasso-<id>/      materialized when module contains plugin/
conventions/             merged from module conventions/
```

## CLI

```bash
lasso module list
lasso module list --installed
lasso module add <id>
lasso module remove <id> --yes
```

## Skills

- `lasso-add-module` / `lasso-remove-module` — guided install/remove
- `lasso-add-language` — maps language names to `lang-*` modules

## Built-in catalog highlights

| Id | Kind | Notes |
|---|---|---|
| `docs-index` | capability | default on init |
| `lang-go` / `lang-rust` / `lang-nix` | convention | language packs |
| `security` | plugin | lightweight scan skill (stub) |
| `hooks` | plugin | Git/worktree safety hooks (config-driven) |
| `npm-release` | plugin | native npm release and package-page skills |
| `pypi-release` | plugin | native PyPI release and project-page skills |
| `cargo-release` | plugin | native Cargo/crates.io release and crate-page skills |

## Authoring a module

```text
modules/<id>/
  module.toml
  conventions/     # optional, merged into workspace
  plugin/          # optional skills + dual manifests (+ hooks/)
  scripts/         # optional helpers
```

Add a `[[module]]` block to `modules/catalog.toml`.
