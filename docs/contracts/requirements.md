# Lasso core requirements

Normative product contract for the Lasso agent-native monorepo workspace.

## LR-1: Agent runtime is the execution kernel

Codex is the design baseline and preferred kernel. Claude Code and Grok Build
are followers. Lasso MUST NOT mirror task, goal, plan, progress, review,
memory, or session state.

## LR-2: Native capability before local implementation

Every core addition must identify native agent coverage, the narrowest extension
surface (AGENTS.md, skill, plugin, hook, MCP), the residual workspace gap, and
a deletion trigger.

## LR-3: Customization uses agent extension surfaces

Prefer AGENTS.md, deterministic gates, plugins, skills, and MCP over a parallel
agent framework. One canonical skill tree; runtime-specific discovery adapters
only.

## LR-4: Core allowlist

Lasso core may own only:

- project identity and stable checkout bootstrap
- state-free worktree placement and GC
- minimal durable work items
- module catalog install/remove into a workspace
- workspace template init
- thin multi-runtime discovery adapters

## LR-5: Modules are incremental

Optional capabilities ship as versioned modules. Workspaces pin them in
`lasso.lock.toml` and may add or remove them without forking core.

## LR-6: Documentation is indexed

Owned `docs/` trees use `docs/INDEX.md` with full Markdown coverage when the
docs-index module or equivalent checker is enabled.

## Acceptance

`just verify` must pass on the product repository. Workspace changes use owner
gates plus `verify-change` when spanning multiple projects or closing work items.
