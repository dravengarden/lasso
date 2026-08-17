---
name: lasso-doctor
description: Diagnose whether the Lasso plugin can find its CLI, workspace root, registry, kit root, and current registered project. Use when Lasso discovery fails or after installing or upgrading the plugin.
---

# Diagnose Lasso discovery

First identify the failing layer. For Codex installation, configuration,
authentication, hooks, plugins, or runtime health, use its native diagnostic
and stop unless a Lasso-specific failure remains:

```bash
codex --strict-config doctor --json
```

For Claude:

```bash
claude doctor
claude plugin list
```

For Grok (when enabled):

```bash
GROK_FOLDER_TRUST=0 grok inspect --json
```

Then run the residual workspace diagnostic:

```bash
"${GROK_PLUGIN_ROOT:-${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}}/scripts/lasso-doctor"
```

Interpret failures by layer:

1. Install `lasso` on `PATH` (build from the product repo with `just build`).
2. Treat an API version mismatch as a stale binary.
3. Set `LASSO_ROOT` when the workspace is not discoverable from cwd.
4. Set `LASSO_KIT_ROOT` when module catalog operations cannot find the product tree.
5. Repair `project-defs/registry.toml` when the registry marker is missing.
6. An unregistered cwd is a normal context result, not a broken plugin.

Do not download binaries or create a second registry as a diagnostic side effect.
