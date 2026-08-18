---
title: Work items
description: Durable coordination that outlives a single agent task.
---

Work items are **not** a second task runtime. Codex / Claude / Grok still own
goals, plans, progress, and review.

Create one only when you need a Git-auditable handoff, external blocker,
migration record, or decision that must survive the current thread.

```bash
lasso work-item new --id=replace-protocol \
  --title="Replace the shared wire protocol without a flag day" \
  --project=app --recipe=migration

lasso work-item list --format=json
lasso work-item validate
lasso work-item rm replace-protocol --yes
```

Skill: **`work-item`** (decides whether an item is warranted and which recipe
artifact to keep).

Never store `plan.md`, `status.md`, or `progress.md` in an item — those
duplicate agent execution state. Promote lasting evidence into the owning
project's docs before closing.
