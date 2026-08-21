# hooks module

Optional Git / worktree safety hooks for Lasso workspaces (Codex, Claude, Grok).

## What it does

- **SessionStart**: read-only machine + Git facts (no fetch)
- **PreToolUse**: block destructive Git and optional NixOS bypasses; fence mutations on stable checkouts

## Configuration

Prefer Lasso env names; Columbus names work during migration:

| Variable | Purpose |
|---|---|
| `LASSO_ROOT` / `COLUMBUS_ROOT` | Workspace root |
| `LASSO_CONFIG` / `COLUMBUS_CONFIG` | JSON config (default `/etc/lasso/config.json`) |
| `LASSO_MACHINE_ID` / `COLUMBUS_MACHINE_ID` | Machine id |

Example config:

```json
{
  "paths": { "root": "/path/to/workspace" },
  "machine": {
    "id": "devbox",
    "nixos_hosts": ["devbox"]
  }
}
```

`nixos_hosts` opts a machine into the committed-source NixOS transaction fence.
Without it, NixOS-specific denies stay inactive.

## Install

```bash
lasso module add hooks
```

Then refresh the agent plugin marketplace and start a new session.
