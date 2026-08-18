---
title: Initialize a workspace
description: Options for lasso init and the generated tree.
---

## Command

```bash
lasso init <path> \
  --name <name> \
  --runtime=codex[,claude][,grok] \
  --module=<id> --module=<id>
```

| Flag | Default | Meaning |
|---|---|---|
| `path` | required | New or empty directory |
| `--name` | directory basename | Workspace display name |
| `--runtime` | `codex` | Agent runtimes to adapt for |
| `--module` | catalog `default = true` | Extra modules to install |

Guided path: invoke the **`lasso-init`** skill.

## Generated layout

```text
lasso.toml                 name + runtimes
lasso.lock.toml            pinned modules
project-defs/registry.toml
projects/
work-items/active/
plugins/lasso-core/
.lasso/modules/            installed module payloads
AGENTS.md
CLAUDE.md                  → @AGENTS.md
.codex/agents/             reviewer adapters
.claude/agents/
.agents/plugins/marketplace.json
.claude-plugin/marketplace.json
docs/INDEX.md
```

## After init

1. `cd` into the workspace
2. Install `lasso-core` into your agent runtime
3. Register projects with `lasso project add`
4. Run `lasso-doctor` if discovery fails
