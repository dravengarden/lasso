---
title: Skills
description: Installable agent skills shipped with lasso-core and modules.
---

## Core plugin (`lasso-core`)

| Skill | Purpose |
|---|---|
| `lasso` | Route general CLI questions |
| `lasso-doctor` | Residual workspace / kit discovery |
| `lasso-init` | Scaffold a workspace |
| `lasso-add-module` | Install a module |
| `lasso-remove-module` | Remove a module |
| `lasso-add-language` | Install `lang-*` packs |
| `project-add` / `project-rm` / `project-update` | Registry lifecycle |
| `work-item` | Durable coordination judgment |
| `verify-change` | Gates + proportional review |

## Module example

| Module | Skill |
|---|---|
| `security` | `security-scan` |

Install plugins from a workspace:

```bash
codex plugin marketplace add .
codex plugin add lasso-core@lasso
```
