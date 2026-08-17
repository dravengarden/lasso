# Operations recipe

Create `runbook.md` only for repeatable operations or changes with meaningful
blast radius. One-off low-risk commands belong in the current thread.

Use `assets/ops.md`. Capture authority boundaries, preflight state, exact
observable success signals, rollback triggers and commands, and post-change
monitoring. Keep credentials out of the repository. Use project or host skills
for execution; the work item holds durable coordination, not secrets or live
service state.
