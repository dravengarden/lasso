---
title: Quick start
description: Create a Lasso workspace and install the core agent plugin in minutes.
---

## 1. Build the CLI

```bash
git clone git@github.com:dravengarden/lasso.git
cd lasso
just build
export PATH="$PWD/bin:$PATH"
export LASSO_KIT_ROOT="$PWD"
lasso version
```

## 2. Initialize a workspace

```bash
lasso init ~/my-fleet \
  --name my-fleet \
  --runtime=codex,claude \
  --module=lang-go
cd ~/my-fleet
```

This writes `lasso.toml`, `lasso.lock.toml`, `project-defs/`, `work-items/`,
`AGENTS.md`, runtime adapters, and vendors `plugins/lasso-core`.

## 3. Install the core plugin

From the **workspace** directory:

```bash
codex plugin marketplace add .
codex plugin add lasso-core@lasso
```

Claude Code / Grok Build followers:

```bash
claude plugin marketplace add .
claude plugin install lasso-core@lasso
```

Start a new agent session after install.

## 4. Register a project

```bash
lasso project add --project=app --repo-url=git@github.com:you/app.git
lasso setup --only=app
lasso project path app
```

Read that checkout's `AGENTS.md` and use its own build commands.

## 5. Add or remove modules

```bash
lasso module list
lasso module add security
lasso module remove security --yes
```

Or ask an agent to use the `lasso-add-module` / `lasso-remove-module` skills.

## Next

- [Initialize options](/lasso/guides/init/)
- [Modules deep dive](/lasso/concepts/modules/)
- [Agent runtimes](/lasso/concepts/agent-runtimes/)
