---
name: security-scan
description: Run a lightweight security review checklist for the current workspace or project. Use when the user asks for a security scan, dependency/supply-chain check, or secret-leak pass before merge.
---

# Security scan

This module ships a lightweight workflow. Extend `references/` or replace the
bundled scripts when you need a heavier scanner.

## Workflow

1. Identify the owning repository and read its nearest `AGENTS.md`.
2. Prefer the project-owned security gate when one exists.
3. Otherwise walk the change set for:
   - hardcoded secrets, tokens, private keys
   - unexpected network egress or shell injection in scripts
   - dependency lockfile churn without justification
   - world-writable files or careless `curl | bash` installers
4. Report concrete file locations. Do not claim a clean bill without saying
   what was and was not exercised.

Deterministic tooling may be added under this module's `scripts/` directory in
later versions; until then, judgment plus project gates are the contract.
