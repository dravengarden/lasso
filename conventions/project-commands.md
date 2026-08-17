# Project-owned commands

The Lasso registry identifies repositories; it does not declare their
install, build, lint, test, run, deploy, API, or health commands.

Resolve the checkout first:

```bash
checkout="$(lasso project path <name>)"
```

Then read that checkout's nearest `AGENTS.md`. Prefer a small project-owned
`justfile` with conventional recipes where the project benefits from them:

- `setup` or `install`
- `build`
- `lint`
- `test`
- `verify` or `check`
- `dev`

Maintained first-party Rust projects additionally follow the recipe contract in
[the Rust development standard](code/rust/rust.md), including dependency
policy, a fast test loop, and opt-in compiler-cache diagnostics.
Maintained first-party Julia projects follow the formatter, lint, inference,
and test gate in [the Julia code guidelines](code/julia/julia.md).

These names are conventions, not Lasso schema. Project-specific skills may
orchestrate additional workflows. Deployments remain under the owning project
or host; Hawk service deployment and health belong to
`machines/hawk/nixos` in an isolated Lasso worktree.

The `verify-change` skill selects the actual gate from `AGENTS.md` and project
commands. A missing aggregate recipe is not treated as a successful skip.
