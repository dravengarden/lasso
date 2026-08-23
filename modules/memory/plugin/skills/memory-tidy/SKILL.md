---
name: memory-tidy
description: Codex-only writer that consolidates, deduplicates, and retires facts across ordinary and DeepSeek-provider Codex memory homes. Claude Code and Grok may only report read-only stale candidates and must hand mutating work to Codex.
disable-model-invocation: true
---

# Memory Tidy

This is a Codex-owned mutating workflow. Follower runtimes may run the
deterministic read-only candidate and stale-path checks, but must stop before
creating a retirement note, moving a backup, or changing any Codex memory home.
Continue that work in Codex.

Tidy is the stronger layer above sync. Sync moves facts between stores (add,
modify, delete propagation); tidy reorganizes a store's content: merge
duplicates, retire superseded facts, drop what consolidation would never keep,
and keep the result reviewable.

Resolve this plugin's `scripts/` directory from the selected skill path. The
deterministic helpers (`tidy-candidates`, `memory-bridge`) live there; command
syntax lives in each script's `--help` or header.

## Invariants

- Run `$memory-sync` first. Tidy on an unsynced store produces merges for
  pairs that sync would already have made identical.
- Never hand-write compiled artifacts. Codex regenerates `MEMORY.md` and
  `memory_summary.md` from rollout summaries and ad-hoc notes; direct edits
  are lost at the next consolidation. Claude and Grok own their native memory
  files and indexes; this plugin never rewrites them.
- Nothing is destroyed. Back up before deleting; use retirement notes where the
  runtime forbids deletion (Codex notes are never deleted).
- Deterministic first, model second. `tidy-candidates` and `memory-sync stale`
  find candidates; the model only merges and judges.
- Consolidated output uses the same Codex write paths as sync: ad-hoc notes
  plus staging for extracted facts. Follower-runtime findings are reports for
  their owning runtime or a human, never plugin write targets.

## Workflow

### 1. Precondition: sync

Run `$memory-sync`: `scan`, `stale`, and the reviewed `extract`/`notes` passes,
then `memory-bridge --write --propagate-updates` between distinct Codex homes
when approved. It never bridges or writes Claude or Grok stores.

### 2. Candidate finding (deterministic)

- `memory-sync stale` — facts referencing paths that no longer exist. Each
  finding is a rewrite-or-retire review item, never an instruction to delete.
- `tidy-candidates` — duplicate and near-duplicate pairs inside each store.
  Default threshold 0.72 finds near-identical word sets; drop to ~0.6 for a
  wider review list. It compares only within the same home and category:
  identical facts copied across homes are the sync contract, not tidy targets.

### 3. Classify candidates

For each pair or group, decide one of:

- exact duplicate — same claim twice in one store: keep canonical, retire the
  other
- near-duplicate — related but distinct: merge only if the union adds a rule
  the reader needs; shared vocabulary is not shared meaning
- superseded — a newer fact contradicts or updates an older one: keep the
  newer, retire the older with a pointer to the replacement
- stale — names a retired path: rewrite to the current location or retire
- low value — execution state, transcripts, incidents without a durable rule:
  retire

### 4. Consolidate (model step)

Merge each group into one durable fact with the A-MEM shape: a standalone
statement, one context line, keywords/tags, and an explicit link to the facts
it supersedes or relates to. Keep what a future agent must know; drop
everything `AGENTS.md` and machine guidance already mandate.

See `references/methods.md` for the surveyed approaches and similarity
thresholds. The agent performing the merge is the judgment layer: no
speculation, no pinned volatile details, temperature-zero discipline.

### 5. Apply writes

Codex:

- add the merged fact as a new ad-hoc note under
  `extensions/ad_hoc/notes/` (name: `YYYYMMDD-HHMMSS-<slug>.md`)
- add a retirement note for each superseded or retired fact stating it no
  longer holds and naming the replacement
- never edit or delete an existing note or rollout summary; Codex's own
  consolidation prunes stale rollout summaries (that is its forgetting)

Claude Code and Grok stores:

- treat plugin findings as read-only review input
- ask the owning runtime or an interactive human to merge or retire a fact
- move deleted files to `~/.cache/lasso-memory/backups/<date>/` first
- let the owning runtime rewrite its own `MEMORY.md`; do not ask the bridge to
  do it

### 6. Verify

- re-run `tidy-candidates` and `memory-sync stale`
- confirm every retirement note names its replacement
- for every candidate pair still above the review threshold, confirm it is
  either intentionally kept (distinct claims) or covered by a retirement note;
  raw note files are never deleted, so the merge takes effect when the
  runtime's next consolidation compiles them
- run `memory-bridge` (dry-run) and confirm it only considers distinct Codex
  homes; it must not report follower-store fact propagation

## Stop conditions

Ask rather than proceeding when:

- a merge would collapse facts that only share vocabulary but make different
  claims
- a whole store appears keyed to an obsolete path; that is fragmentation to
  fix at the source, not by deleting facts
- retirement would lose a fact with no replacement and no backup
- the gateway is unreachable; do not fall back to a direct provider call or
  introduce a credential

## References

- `references/methods.md` — memory engineering survey and merge/dedup guidance
- `references/codex-internals.md` — store map, jobs, config, and forgetting
  semantics
