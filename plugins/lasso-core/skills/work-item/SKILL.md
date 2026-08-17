---
name: work-item
description: Decide whether engineering work needs a repository-auditable Lasso work item, then create, resume, block, close, or remove its minimal metadata and recipe-specific artifacts. Use when the user names work-items/, when another person/runtime needs a durable handoff, or when an external blocker, migration, operational runbook, or formal decision must remain independently inspectable in Git. Do not create one merely because work spans projects or agent sessions.
disable-model-invocation: true
---

# Work item

Treat a work item as durable coordination, never as a second agent runtime.
Codex owns the baseline live task lifecycle. When Claude follows, its equivalent
goal, plan, progress, conversation, and review remain Claude-local as well.

## Decide whether to persist

Create an item only when the durable repository artifact itself has value and at
least one condition holds:

- Another person, runtime, or independently created agent task needs a handoff
  that cannot rely on the current thread.
- An external dependency or blocker must remain visible outside runtime state.
- A migration, repeatable/risky operation, or formal design decision needs an
  auditable Git home.
- The user explicitly requests a durable item under `work-items/`.

Otherwise, use the current runtime task and native goal surface; use its plan
surface when execution is unclear. Codex and current Claude Code both provide
native goal ownership. Multiple projects, sessions, worktrees, or
implementation phases are not sufficient reasons. Do not create filesystem
state merely to mirror runtime state.

## Create

1. Select the closest recipe below and read exactly that reference. Project-local
   skills take precedence when they define a more specific workflow.
2. Inspect central items projected for the current registered project:

   ```bash
   lasso work-item list --current --format=json
   ```

3. Create metadata without a worktree. Omit `--project` when cwd uniquely
   identifies the intended project; repeat it only for explicit cross-project
   scope:

   ```bash
   lasso work-item new --id=<id> --title=<outcome> \
     --project=<project> [--project=<project>] --recipe=<recipe>
   ```

4. Copy the referenced asset only when its document will remain useful after
   the current thread. Rename it to the recipe's artifact name, replace every
   placeholder, and delete irrelevant sections.
5. Run `lasso work-item validate`.

Recipes are open vocabulary; these references are guidance, not a schema enum:

| Recipe | Read | Durable artifact |
|---|---|---|
| feature | `references/feature.md` | `brief.md` when scope needs a handoff |
| bug | `references/bug.md` | `investigation.md` for durable evidence/root cause |
| spike | `references/spike.md` | `findings.md` when a decision consumes the result |
| design | `references/design.md` | `decision.md` |
| migration | `references/migration.md` | `migration.md` |
| ops | `references/ops.md` | `runbook.md` for repeatable or risky operations |

Do not force a recipe document onto simple work. Never leave template markers
in an active item.

## Resume

1. Run `lasso work-item show <id>` and `lasso work-item path <id>`.
2. Read only its relevant durable artifacts and the owning projects' nearest
   `AGENTS.md` files.
3. Set the active runtime's goal from the desired outcome; create a live plan
   only if needed. Do not infer progress from old prose or add lifecycle fields.
4. Use the stable Local checkout only for discovery/read-only inspection; use a
   runtime-owned task worktree for mutations. Metadata is never inferred from
   the branch name.

Edit `blocked_reason` only when it describes a current durable external fact.
Describe related work with ordinary links in the recipe artifact; never build
an execution graph in metadata. Validate after editing metadata.

Cowboy and Zed may project matching central items for discovery, but neither
owns lifecycle state. Selecting an item starts or steers the supported native
agent task; it never binds the item to that UI session.

## Close or delete

- Move a lasting decision, migration record, investigation, or runbook into the
  owning project's documentation before closing the work item.
- After user approval, close an item with `lasso work-item rm <id> --yes`.
  This removes only the active working-tree copy; Git history retains the prior
  coordination record and no checkout or agent worktree is touched.
- Do not create a Lasso archive. An artifact that must remain visible in the
  current tree belongs in the owning project's docs, not a historical work-item
  directory.
- Verify implementation through the `verify-change` skill before claiming the
  outcome is complete. Deletion or document location is not proof of completion.
