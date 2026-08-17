# Review convention

Follow the review rules in [`../AGENTS.md`](../AGENTS.md). Deterministic gates
run before review; review adds scoped judgment and never replaces a failing or
missing owner gate. Review only the requested diff, preserve unrelated dirty
changes, cite concrete file locations, and ignore style preferences already
enforced mechanically.

A reviewer runs read-only and keeps persistent runtime memory off. A review
must be reproducible from the requested diff and tracked repository guidance
alone; a retained verdict from an earlier review would seed the next one.

## Code reviewer

Prioritize correctness bugs, security risks, behavioral regressions, and
missing high-value tests. Trace affected execution paths. Do not edit files or
rerun broad implementation work. If no material finding exists, say so and
name the residual untested boundary.

## Design reviewer

Compare the change with repository guidance, durable design artifacts, and the
stated acceptance outcome. For Lasso core changes, enforce
[`../docs/requirements.md`](../docs/requirements.md): identify native Codex
coverage, the narrowest extension surface, the concrete residual gap, the
deletion trigger, and the CR-7 Claude follower classification. Treat
unexplained duplication or compatibility drift as blocking. Look for missing
constraints, incompatible transitions, unsafe rollout ordering, and docs that
no longer match behavior. When a change can alter model-visible context,
enforce CR-9 by distinguishing required semantic invalidation from accidental
prefix drift and requiring proportionate cache non-regression evidence. Do not
turn optional improvements into blockers.
