---
name: verify-change
description: Verify Lasso core changes, coordinated changes spanning multiple registered repositories, or closure of a durable work item with each owner's deterministic quality gate and risk-proportional review. Codex is the baseline and Claude Code follows through native review or thin reviewer adapters. For an ordinary single-project change, follow that project's nearest guidance directly.
---

# Verify change

Treat command evidence as the gate and review as independent judgment. A stored
status, plan checkbox, or archive action is never proof.

## Establish scope

1. Use this skill only for a Lasso core change, multiple owning repositories,
   or durable work-item closure. For an ordinary single-project change, stop and
   follow that project's nearest guidance directly.
2. Inspect the diff and identify every owning repository. Preserve unrelated
   dirty changes and verify each repository independently.
3. Read the nearest `AGENTS.md` and project-specific skill for any domain gate.
4. Classify risk using `references/risk.md`.
5. For a Lasso core change, read `docs/requirements.md` and record the native
   Codex coverage, chosen extension surface, residual gap, deletion trigger, and
   CR-7 Claude follower classification. Reject the change if it duplicates
   Codex without a concrete gap or leaves the follower path unexplained.

## Run deterministic gates

Prefer the project-owned contract in this order:

1. The exact command required by the nearest `AGENTS.md`.
2. `just verify`.
3. `just check`.
4. Targeted native build, lint, and test commands when no aggregate recipe
   exists.

Lasso core changes additionally run `just architecture-check`; it is a
non-regression check, not a substitute for the capability-admission review.

Resolve the stable checkout with `lasso project path <name>`
when needed, then run its project-owned command. Do not accept a missing or
skipped gate as success.

Run targeted tests early for feedback, then the required full gate. Record the
exact failing command and first actionable cause. Fix in-scope failures and
rerun; distinguish pre-existing unrelated failures with concrete evidence.

## Review proportionally

- Low risk: inspect the complete diff and targeted test evidence in the current
  thread. Do not spawn a reviewer by default.
- Medium risk: run the full project gate and use Codex's native review surface
  for the working tree or base diff. In Claude, use `/review`,
  `/ultrareview`, or the follower reviewer agent over the same scoped diff.
- High risk: after deterministic gates pass, ask for focused independent
  read-only review. Prefer the project custom `design_reviewer` for contract and
  acceptance drift and `code_reviewer` for correctness/security/test gaps.
  Parallelize only independent read-heavy review; never require fixed reviewer
  count for every change.

Review findings must name a concrete behavior risk and file location. Ignore
style-only comments already covered by formatters unless they conceal a defect.

## Report

State:

- Repositories and diff scope verified.
- Commands run and whether each passed.
- Manual or live-system checks performed, if any.
- Review findings fixed or remaining.
- Any unverified boundary and why it could not be exercised.

Store evidence in a work item only when it is useful for a future handoff,
audit, migration, or operator. Otherwise keep it in the active runtime's task
response.
