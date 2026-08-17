---
name: project-rm
description: Unregister a project from the Lasso harness while leaving its checkout and documentation untouched. Use when the user explicitly asks to deprecate or remove a registered project.
disable-model-invocation: true
---

# Remove a project registration

1. Resolve the exact name with `lasso project list --format=json`.
2. Inspect active work-item references, the stable checkout, agent-runtime worktrees, uncommitted changes, and unpushed commits. The command removes only registry metadata; it does not delete `projects/$NAME/` or documentation.
3. Run:

   ```bash
   lasso project rm --project="$NAME" --yes
   ```

4. Use `--force` only after explicit confirmation that active work items may be left pointing at a missing project.
5. Run `lasso work-item validate`, then report remaining checkouts, agent worktrees, and repositories separately.

Deleting the checkout, agent worktrees, remote repository, remote branches, or deployed service is outside this command and requires separate authorization.
