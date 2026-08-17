---
name: project-update
description: Reconcile registered Lasso project metadata and stable checkout status with upstream Git remotes. Use when the user asks to sync a project, refresh its default branch, fetch its stable checkout, or check whether projects are current.
disable-model-invocation: true
---

# Reconcile projects

Use one project or all projects explicitly:

```bash
lasso project update --project="$NAME"
lasso project update --all-projects
```

For a read-only status request, add `--check`. Add `--no-fetch` as well only
when even remote-tracking ref refresh is outside scope or offline behavior is
required.

The default operation fetches remote refs, detects default-branch drift,
reports whether refs were freshly fetched or cached, and reports the stable
checkout's dirty, branch, ahead/behind, or diverged state. It reconciles the
project TOML when necessary. Documentation remains project-owned.

If upstream changed its default branch, the CLI evaluates checkout status
against the detected branch before reconciling registry metadata. It never
switches branches implicitly; a checkout still on the old branch is reported
as wrong-branch and remains untouched.

Use flags deliberately:

- `--no-fetch` for an offline, cached-only inspection.
- `--check` to report without changing TOML or fast-forwarding the checkout.
- Fast-forward is on by default and can be disabled with
  `--pull-ff-only=false`. After fetching, it fast-forwards only a clean
  checkout on the detected default branch; dirty, detached, wrong-branch,
  ahead, and diverged states remain untouched.

Never raw pull, merge, rebase, reset, force-push, auto-stash, or discard a
dirty checkout as an inferred part of reconciliation. Report divergence and
let the user choose the task or integration worktree strategy.
