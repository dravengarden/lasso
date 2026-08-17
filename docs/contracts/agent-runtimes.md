# Agent runtimes

| Runtime | Role | Discovery |
|---|---|---|
| Codex | Kernel | `AGENTS.md`, `.codex/`, Codex plugin manifests |
| Claude Code | Follower | `CLAUDE.md` → `AGENTS.md`, `.claude/`, `.claude-plugin/` |
| Grok Build | Follower | Claude-compatible plugin tree; no third skill fork |

Classify every runtime-dependent change as **shared**, **adapter**, or
**intentional gap**. Never synchronize transcripts, goals, or memories across
runtimes.
