---
name: lasso-remove-module
description: Remove an installed Lasso module from the current workspace, update the lockfile, and refresh marketplace manifests. Use when the user wants to disable or uninstall a capability pack.
disable-model-invocation: true
---

# Remove a Lasso module

## Preconditions

- Cwd is inside a Lasso workspace.
- User explicitly confirmed the module id to remove.

## Workflow

1. Show installed modules:

   ```bash
   lasso module list --installed --format=json
   ```

2. Refuse to remove `core` (not a module; part of init).
3. Warn when other installed modules require this id; the CLI blocks that case.
4. Remove only after confirmation:

   ```bash
   lasso module remove <id> --yes --format=json
   ```

5. Report lockfile and marketplace updates. Mention any conventions that were
   best-effort deleted and any leftover files the user may want to prune.

## Safety

- Never pass `--yes` without clear user intent to remove that module.
- Do not delete unrelated workspace projects, work items, or checkouts.
