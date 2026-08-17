---
name: project-add
description: Register an external Git repository in the Lasso harness and optionally create its stable ordinary checkout. Use when the user asks to onboard, register, or track a new repository.
disable-model-invocation: true
---

# Add a project

1. Confirm the registry name and remote URL. The name must be a stable lowercase slug.
2. Check for collisions:

   ```bash
   lasso project list --format=json
   test ! -e "project-defs/$NAME"
   ```

3. Let the CLI discover the remote default branch. Supply `--default-branch` only when the user intentionally wants an override.
4. Run:

   ```bash
   lasso project add --project="$NAME" --repo-url="$URL"
   ```

   Use `--no-clone` only for an intentional registry-only add.
5. Inspect the generated `project-defs/$NAME/project.toml` and stable checkout. Project
   documentation belongs to the registered repository and is not scaffolded by
   this command. When the checkout contains a `docs/` tree, Lasso's
   `just docs-index-check` validates its shared index shape and coverage, while
   the project owns the content and commits. Run `just verify` in Lasso;
   project setup and verification remain project-owned.
6. For a Lasso-owned repository, classify its agent surfaces under CR-7:
   keep `AGENTS.md` and `.agents/skills/` canonical, add thin Claude discovery
   adapters when those surfaces exist, and classify plugin/hook differences or
   intentional gaps. Do not modify a third-party repository merely to satisfy
   Lasso compatibility policy; record the external ownership boundary.

This skill handles external repositories. For a new in-tree subdirectory project, design and register the `kind: "subdir"` entry directly because the current add command is remote-oriented.

Do not guess credentials, create a remote repository, push, or deploy unless separately requested.
