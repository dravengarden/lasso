---
name: lasso-init
description: Create a new Lasso agent-native monorepo workspace with chosen runtimes and optional modules. Use when the user wants to scaffold, bootstrap, or initialize a new multi-project agent workspace.
disable-model-invocation: true
---

# Initialize a Lasso workspace

## Preconditions

1. `lasso` is on `PATH` (`just build` in the Lasso product repo).
2. `LASSO_KIT_ROOT` points at the Lasso product checkout, or the shell cwd can
   walk up to `modules/catalog.toml`.

## Decide options

Ask only for missing choices:

| Option | Default | Notes |
|---|---|---|
| path | required | empty directory or new path |
| name | directory basename | workspace display name |
| runtimes | `codex` | add `claude` and/or `grok` when requested |
| modules | catalog defaults | e.g. `docs-index`; add `lang-go`, `security`, … |

List modules when the user is unsure:

```bash
lasso module list --format=json
```

## Create

```bash
lasso init <path> \
  --name=<name> \
  --runtime=codex[,claude][,grok] \
  --module=<id> --module=<id>
```

Confirm the printed path, runtimes, and installed modules. Then:

1. `cd` into the workspace.
2. Install the core plugin for the user's runtimes:

   ```bash
   codex plugin marketplace add .
   codex plugin add lasso-core@lasso
   ```

   Claude / Grok followers:

   ```bash
   claude plugin marketplace add .
   claude plugin install lasso-core@lasso
   ```

3. Optionally register the first project with the `project-add` skill.
4. Run `lasso-doctor` if discovery fails.

## Stop conditions

- Refuse to init into a non-empty non-workspace directory.
- Do not copy personal machine inventories, secrets, or host-specific paths.
- Do not install modules the user did not request beyond catalog defaults.
