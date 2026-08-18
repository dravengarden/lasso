---
title: Agent runtimes
description: Codex kernel with Claude Code and Grok Build followers.
---

## Roles

| Runtime | Role | Discovery |
|---|---|---|
| **Codex** | Execution kernel / design baseline | `AGENTS.md`, `.codex/`, Codex plugin manifests |
| **Claude Code** | Primary follower | `CLAUDE.md` → `AGENTS.md`, `.claude/`, `.claude-plugin/` |
| **Grok Build** | Additional follower | Claude-compatible plugin tree (no third skill fork) |

## Rules

1. **One canonical implementation** — skills, hooks, review contracts written once.
2. Classify every runtime-dependent change as **shared**, **adapter**, or **intentional gap**.
3. **Never** synchronize transcripts, goals, plans, memories, or session state across runtimes.
4. Grok does not get a separate `plugin.json` tree in v0.1 — it consumes the Claude-compatible surface.

## Reviewers

Shared contract: `conventions/review.md`  
Adapters: `.codex/agents/*` and `.claude/agents/*`
