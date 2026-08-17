# Agent skill conventions — Codex first

Skills encode reusable judgment and workflows that are not already guaranteed
by a CLI, schema, test, or hook. Keep command syntax in the command's `--help`;
keep the skill focused on sequencing, decisions, safety gates, and recovery.

Skills are authored and optimized for Codex workflows. Package them as a Codex
plugin when they need installable distribution, companion MCP/app
configuration, or a reusable bundle. When Claude Code can safely follow, add
only its discovery manifest or symlink around the same skill tree; do not fork
the workflow or invent a Lasso-specific installer or runtime.

## Ownership

Place a skill in the narrowest repository that owns the workflow:

- The `lasso-harness` plugin owns installable harness, project
  catalog/stable-checkout, durable work-item, and cross-project verification skills.
  Do not also expose those skills under `.agents/skills/`; duplicate discovery
  creates ambiguous triggers and stale installed-cache behavior. Optional
  capabilities such as security scanning are distributed by their own plugin.
- A project owns workflows that teach its own commands and contracts.
- `machines/<id>/nixos` owns host operations and truly machine-wide skills.
- Fan out only skills useful across unrelated repositories. Codex receives the
  primary `~/.codex/skills/` entry; Claude receives a follower
  `~/.claude/skills/` symlink to the same canonical directory when compatible.

Do not duplicate skill content into tool-specific wrapper trees. A distributed
plugin skill is canonical under its plugin and is not projected into a second
repo-local discovery path. A repository-only skill is canonical in the nearest
`.agents/skills/`.
Do not link a project-owned skill into Lasso `.agents/skills/`; enter that
project or package its capabilities as a Codex plugin when installable global
discovery is genuinely required.

## Layout

```text
<skill>/
├── SKILL.md
├── agents/openai.yaml       # Codex UI metadata; harmless to other runtimes
├── scripts/                 # deterministic executable helpers
├── references/              # details loaded only when needed
└── assets/                  # templates or output assets
```

Create only directories the skill uses. Do not keep README, changelog,
`__pycache__`, test cache, generated output, or empty legacy helper directories
inside a skill.

## Frontmatter

Use exactly `name` and `description`:

```yaml
---
name: verify-change
description: Verify implemented changes with the owning project's deterministic quality gate and risk-proportional Codex review. Use after implementation or before committing and claiming completion.
---
```

The description is the trigger. Include the scope and concrete user intent
there. Do not use `trigger`, `user-invocable`, `$ARGUMENTS`, slash-command
syntax, or angle-bracket placeholders.

`disable-model-invocation: true` is an optional third key (value must be
exactly `true`), reserved as an explicit exception for skills whose invocation
itself has real side effects: destructive operations, state mutation, deploys
or rollouts, or durable record writes (registry, memory, verdict, or lockfile
changes). It is a Claude-only adapter field under CR-7: Claude Code reads it
from the frontmatter to require explicit `/<name>` invocation instead of
automatic description matching, while Codex ignores it (its loader drops
unrecognized frontmatter keys). It never changes canonical workflow content
and never replaces Codex's own invocation policy, which Codex reads from
`agents/openai.yaml` (`policy.allow_implicit_invocation`, supported by Codex
0.146+ but not used by any Lasso skill yet); it must not be applied to
read-only or diagnostic skills. Any other extra frontmatter key remains a
validation failure; keep the default exactly `name` then `description`.

```yaml
---
name: project-rm
description: Unregister a project from the Lasso harness while leaving its checkout and documentation untouched.
disable-model-invocation: true
---
```

## Body

- Use imperative language.
- Assume the active agent knows general software engineering.
- Prefer a short workflow plus explicit stop conditions.
- Reference bundled files directly and say when to read or execute them.
- Use CLI structured output instead of parsing human tables.
- Keep destructive operations behind explicit scope confirmation.
- Use the active runtime's native plans, managed sessions, and subagents where
  they materially improve the workflow; optimize and test for Codex first.
- Classify any Codex-only dependency as shared, adapter-backed, or an
  intentional Claude gap under CR-7 before publishing the skill.
- Move incident histories and large command catalogs to `references/`.

## Validation

Run:

```bash
bash scripts/check-skills.sh
```

The repository quality gate runs the same check. After a substantial or risky
change, forward-test the skill with a fresh-context Codex subagent using a
realistic task and no leaked expected answer.
