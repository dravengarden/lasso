# Verification risk

Choose the highest matching level.

## Low

- Documentation or comments with no executable contract change.
- Local refactor with narrow types and direct unit coverage.
- Generated or formatting-only changes whose source is unchanged.

Use targeted checks plus complete diff inspection.

## Medium

- User-visible behavior, public API, schema, dependency, or persistence change.
- Multiple packages in one repository.
- Build, packaging, or project workflow changes.

Run the repository-wide gate and native review. Codex is the baseline; Claude
uses its native review surface or follower reviewer agent over the same diff.

## High

- Authentication, authorization, secrets, money, destructive data operations,
  concurrency, protocol compatibility, supply chain, or sandbox boundaries.
- Cross-project rollout or migration with an old/new compatibility window.
- NixOS service composition, deployment, networking, DNS, firewall, routing,
  TUN, or other changes with host blast radius.

Run every owning project's full gate, required isolated/live checks, and focused
read-only independent review. Follow explicit approval boundaries before any
deployment or external mutation.
