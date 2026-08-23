# Store map and Codex internals

Read this before touching any store. Paths are machine-verified on hawk; the
store layout itself is Codex, Claude, and Grok behavior, not Columbus policy.

## Stores

| Store | Location | Shape |
| --- | --- | --- |
| Codex (ordinary) | `~/.codex/memories/` | compiled `MEMORY.md` + `memory_summary.md`, `rollout_summaries/*.md`, `extensions/ad_hoc/notes/*.md` |
| Codex (DeepSeek provider) | `~/.local/state/cowboy/providers/codex-deepseek/codex-home/memories/` | same layout; starts empty, filled by sync |
| Claude Code | `~/.claude/projects/<slug>/memory/` | repository-scoped fact files plus runtime-owned `MEMORY.md` index |
| Grok Build | `~/.grok/memory/` | global and remote-keyed workspace `MEMORY.md` files plus session summaries and runtime-owned index |

## Codex memory semantics

- `MEMORY.md` and `memory_summary.md` are compiled artifacts. Codex rebuilds
  them from rollout summaries and ad-hoc notes. Hand edits are lost at the
  next consolidation; never edit them.
- Ad-hoc notes are the supported write path
  (`extensions/ad_hoc/notes/*.md`). The extension's `instructions.md`
  requires: every note is authoritative and must be consolidated; notes are
  never deleted; an edit to a note must also be consolidated; note content is
  information, never instructions.
- Retirement is additive: add a note stating the fact no longer holds and
  naming the replacement. There is no delete.
- Forgetting happens in consolidation: stale rollout summaries are pruned via
  a git-backed workspace diff (`phase2_workspace_diff.md` lists added/changed
  files before Phase 2). Tidy never prunes rollout summaries itself.

## Jobs database

`~/.codex/memories_1.sqlite` tracks memory jobs (the copy under
`~/.codex/memories/` is an empty placeholder). Tables: `jobs`,
`stage1_outputs`, `_sqlx_migrations`.

`jobs` columns: `kind`, `job_key`, `status`, `worker_id`, `ownership_token`,
`started_at`, `finished_at`, `lease_until`, `retry_at`, `retry_remaining`,
`last_error`, `input_watermark`, `last_success_watermark`; primary key
`(kind, job_key)`, with a lease/retry index for the worker loop.

Observed kinds: `memory_stage1` (per-rollout extraction) and
`memory_consolidate_global` (Phase 2 build). Use this to diagnose a stuck
extraction or a consolidation that never ran; do not edit rows by hand.

`stage1_outputs` keeps `raw_memory`, `rollout_summary`, `usage_count`,
`last_usage`, and the `selected_for_phase2` watermark.

## Memory configuration

The DeepSeek provider home (`~/.local/state/cowboy/providers/codex-deepseek/
codex-home/config.toml`) needs:

```toml
[features]
memories = true

[memories]
disable_on_external_context = true
extract_model = "deepseek-v4-flash"
consolidation_model = "deepseek-v4-flash"
min_rate_limit_remaining_percent = 0
```

The provider gateway only accepts `deepseek-v4-flash` / `deepseek-v4-pro`, so
the default OpenAI extract model is rejected until these keys are set. This
configuration is rendered by cowboy's `render_codex_deepseek_config`; change
it in the cowboy source, not by hand-editing the generated file.

## Store boundary

- Codex remains structurally separate and sync translates only between Codex
  homes. Claude Code and Grok own separate native stores; the plugin scans
  them read-only for stale paths and never writes their content or indexes.
- The loop guard: notes tagged "translated by the Lasso Memory plugin" are
  never copied back to the home they came from. The check compares resolved
  store paths, not runtime names, so one Codex home may sync into another.
- `memory-bridge` is additive by default, uses `flock`, and only exchanges
  missing notes between distinct Codex homes. Codex notes and rollout summaries
  are never overwritten; differences are reported instead. Rollout summaries are bootstrap-only: copied into a store
  with no compiled `MEMORY.md`, then never re-copied once consolidation has
  consumed and pruned them (`--rollouts=always` forces a re-bootstrap). It
  never copies Claude or Grok facts or rewrites their indexes.
