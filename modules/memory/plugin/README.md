# lasso-memory

Moves durable facts between Codex homes and scans Claude Code and Grok memory
stores read-only for stale filesystem references.

## Why this exists

Codex compiles a task-group narrative. Ordinary and DeepSeek Codex homes share
that shape, so the plugin can bridge their additions and report differences.
Claude Code and Grok instead own separate native stores. The plugin discovers
their repository/workspace memory and Grok session-summary locations only for
deterministic stale-path scanning; it has no extraction, write, bridge, or
index path for the follower runtime. `source_home` remains the resolved
physical origin used by the Codex loop guard.

## Ownership

Codex owns memory compilation; Claude Code and Grok own their native stores.
Delete this plugin when Codex homes no longer need translation and stale
follower-store audits, or when the durable rules it moves have all been promoted
into checked-in guidance, which is the better home for anything that must
always be in force.

Core `harness-cli` deliberately does not gain these commands: it must not
depend on a model provider.

## Design rules

These are enforced in code and covered by tests, not left to a prompt.

- The model proposes candidates and judges durability, scope, and whether
  repository guidance already covers a statement. It never decides which of two
  statements is newer, and it is told so explicitly. Ordering comes from the
  position of a task group in the Codex artifact.
- Volatile candidates are dropped in code. A first run against the real corpus
  kept 16 of 16 proposals, including a fact quoting two loopback ports and a
  Rust symbol that the prompt had explicitly asked it to reject. Ports,
  addresses, revisions, versions, source symbols, and store paths are now a
  pattern match, not a judgment.
- Guidance coverage is retrieved, not filtered. Lexical overlap was tried as a
  filter first and failed in both directions: a real restatement of the
  deployment rule shared three content words with the paragraph mandating it
  and scored 0.16, while a threshold low enough to catch it flagged half the
  batch. Overlap now only ranks which guidance statements to repeat next to the
  input.
- Requests set `temperature: 0` and disable thinking. Thinking is billed against
  the output budget and truncated structured answers mid-object.
- Contradiction detection runs before deduplication. A dedup gate placed first
  drops the conflicting write before anything can observe the conflict.
- Nothing is deleted. Stale references are reported for review.
- A fact never returns to the store it came from. The check compares resolved
  store paths, not runtime names, because two Codex homes exist here — the
  ordinary one and the DeepSeek provider's — and a sync between them is
  legitimate while a write back into the origin is the loop. Provenance records
  the store; `build_notes` requires the destination, so the filter and the
  writer cannot drift apart; and `write_notes` re-checks the pairing and raises
  rather than writing somewhere the notes were not filtered for.
- The system prefix is byte-identical across a batch, so DeepSeek's prefix
  cache can match it. Per-call state stays in the user message.

## Known limits

`covered_by_guidance` does not work yet. Across three runs over the same two
task groups, V4 Flash returned it false for every proposal, including a rule
whose mandating paragraph was retrieved and placed directly beside the input.
Reviewing staged facts against `AGENTS.md` is therefore a manual step, and the
skill says so. The next thing to try is a separate single-purpose call per
candidate rather than one call that both extracts and judges.

`temperature: 0` did not make runs reproducible either: the same two groups
yielded 13, 15, and 17 proposals. Stability across runs comes from the
deterministic layer — name-keyed merging and content digests — not from the
model.

## Credentials

None. `memory-sync` posts to a host DeepSeek gateway on loopback, which
replaces client authorization with its own systemd credential and reports
content-free usage. The endpoint is rejected unless it is loopback.

`codex-deepseek` and `claude-deepseek` document themselves as boundaries for
their own runtime. This plugin is a second consumer of that protocol boundary,
not of their agent state; its usage is attributed to whichever gateway it is
pointed at.

## Verification

```bash
nix develop -c just verify
```

## Two skills, one pipeline

- `memory-sync` propagates additions, edits, and retirements between Codex
  homes: `memory-sync scan/extract/notes` for reviewable fact movement,
  `memory-sync stale` for read-only scanning across Codex staging plus follower
  stores, and `memory-bridge --write --propagate-updates` only for additive
  exchange between distinct Codex homes. Rollout summaries bootstrap only:
  they are copied into a store with no compiled `MEMORY.md`, then never
  re-copied once consolidation has consumed them. Claude and Grok are never
  bridged or written by this plugin.
- `memory-tidy` runs on top of a synced Codex state: `tidy-candidates` finds
  duplicate and near-duplicate facts inside each store (Jaccard on normalized
  word sets, same-home pairs only). Codex findings may be merged through its
  additive note path; Claude and Grok findings remain read-only reports for
  their owning runtime or an interactive human. Nothing is deleted without a
  backup or a retirement note.

Both skills live in `skills/` and are loaded by the plugin's `skills: ./skills/`
entry.

## Cost

A full pass over a 105 KB Codex memory artifact is a few cents on V4 Flash.
The prefix cache is worth roughly 80% of a per-group fan-out, which buys the
higher-quality shape rather than a meaningful saving. Incremental runs are
sparse enough that the cache has usually expired, and cheap enough that it does
not matter.
