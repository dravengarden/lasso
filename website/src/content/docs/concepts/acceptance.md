---
title: Acceptance
description: Deterministic gates first, then proportional review.
---

## Three layers

| Layer | What | Example |
|---|---|---|
| **L0** | Mechanical, CI-able | `just verify`, `lasso work-item validate`, skill frontmatter checks |
| **L1** | Workflow evidence | `verify-change` skill — run owner gates, record commands |
| **L2** | Risk-proportional review | native review / `code-reviewer` / `design-reviewer` |

Iron rules:

- Stored status / plan checkboxes are **not** completion proof
- Skipping a gate ≠ pass
- Review never replaces a failing L0

## Product gate

```bash
just verify
```

## Workspace / multi-project changes

Use the **`verify-change`** skill after implementation.
