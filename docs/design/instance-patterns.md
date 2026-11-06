# Patterns from workspace instances

Columbus fleet guidance and Suger project orchestration expose the same ownership
boundaries. Adopt the boundaries, not their host names or business commands.

| Experience | Reusable pattern | Owner |
|---|---|---|
| Columbus separates fleet configuration from project commands | Registry records identity; execution follows the resolved owner's contract | Core discovery, instance routing |
| Both instances use stable checkouts and task worktrees | Discover through stable paths, mutate and verify the isolated checkout | Native runtime and state-free worktree helper |
| Suger keeps personal tooling outside company repositories | External projects remain independently usable | Workspace template and project review |
| Suger isolates test databases and ports; Columbus deploys exact commits | Git isolation alone does not isolate external state | Project or machine environment tooling |
| Both instances require fresh refs before integration | Fetch failures and dirty checkouts must remain visible | Owner's integration workflow |
| Instances have different push and deploy rules | Never infer authorization from registration | Owner policy |

## Product changes

The workspace AGENTS template carries these boundaries to new instances.
The shared verify-change skill checks the actual task checkout and resolves
contracts and gates from their owner instead of assuming Columbus paths exist
in the product. Existing instances adopt template guidance deliberately;
upgrading Lasso must not overwrite their local policy.

## Admission and compatibility

Native runtimes retain execution, review, approvals, and task worktrees. The
narrow extension surfaces here are AGENTS.md and a shared skill. The residual
gap is durable repository ownership and cross-project verification guidance;
no task state or environment manager is added. Claude and Grok consume the same
policy through existing discovery adapters. Remove duplicate workflow guidance
when native tooling enforces the same owner-aware boundaries; retain project
identity and instance-specific policy where still required.

Environment provisioning, browser profiles, account selection, and deployment
execution stay outside core. Extract an optional module only after independent
instances share a concrete interface, lifecycle, and deterministic gate.
