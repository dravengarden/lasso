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
   this command. Run the workspace's owning registry gate in the workspace
   containing the change. Run a docs-index gate only where it is installed and
   required; project documentation, setup, verification, and commits remain
   project-owned. Do not run the Lasso product gate as a substitute for checking
   an instance's registry change.
6. For a workspace-owned repository, classify runtime-dependent surfaces as
   shared, adapter-backed, or an intentional gap with a documented fallback:
   keep `AGENTS.md` and `.agents/skills/` canonical, add thin Claude discovery
   adapters when those surfaces exist, and classify plugin/hook differences or
   intentional gaps. Do not modify a third-party repository merely to satisfy
   Lasso compatibility policy; record the external ownership boundary.

This skill handles external repositories. For a new in-tree subdirectory project, design and register the `kind: "subdir"` entry directly because the current add command is remote-oriented.

Do not guess credentials, create a remote repository, push, or deploy unless separately requested.
