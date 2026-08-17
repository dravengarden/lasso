---
name: lasso-add-module
description: Install an optional Lasso module into the current workspace and refresh marketplace manifests. Use when the user wants to add language packs, security, docs checking, or other capability modules.
disable-model-invocation: true
---

# Add a Lasso module

## Preconditions

- Cwd is inside a Lasso workspace (`project-defs/registry.toml`, `lasso.toml`).
- `LASSO_KIT_ROOT` is set or the kit is discoverable.
- `lasso` is on `PATH`.

## Workflow

1. List available and installed modules:

   ```bash
   lasso module list --format=json
   lasso module list --installed --format=json
   ```

2. Confirm the module id with the user when ambiguous.
3. Install:

   ```bash
   lasso module add <id> --format=json
   ```

4. If the module materializes a plugin, remind the user to reinstall or refresh
   the marketplace plugin in their agent runtime and start a new session.
5. Summarize what changed: lockfile pin, `.lasso/modules/<id>/`, conventions or
   `plugins/lasso-<id>/` when present.

## Notes

- Dependencies declared in the catalog must be installed first; the CLI enforces
  this.
- Language packs (`lang-*`) only merge conventions; they do not add plugins.
- Prefer one module per invocation unless the user lists several explicitly.
