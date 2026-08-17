# Lasso Core plugin

Codex-first plugin for the Lasso agent-native monorepo workspace. Claude Code
and Grok Build follow through the shared skill tree and the Claude plugin
manifest.

## Install from a workspace that vendors this plugin

```bash
codex plugin marketplace add .
codex plugin add lasso-core@lasso
```

Claude / Grok:

```bash
claude plugin marketplace add .
claude plugin install lasso-core@lasso
```

## Host dependency

`lasso` API 2, Git, Bash, and Python 3 on `PATH`. Set `LASSO_ROOT` when the
workspace is not discoverable from cwd. Set `LASSO_KIT_ROOT` when installing
modules from the product repository.
