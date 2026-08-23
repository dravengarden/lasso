# Memory engineering survey

How frontier memory systems decide what to keep, merge, and forget, and what
tidy borrows from each. Read this when classifying candidates or writing a
merged fact. The sync/bridge scripts stay the write path; this file only
informs judgment.

## A-MEM: Zettelkasten memory with evolution (NeurIPS 2025, arXiv 2502.12110)

- Every memory is a rich structured note: a context description, keywords,
  and tags.
- New memories are linked to related existing ones; linking is agent-driven
  and builds an interconnected note network.
- A new experience can update an existing note (memory evolution) instead of
  appending yet another variant.

Takeaway: a merged fact should carry context, keywords/tags, and explicit
links to the facts it supersedes or relates to. Consolidation is an update,
not a delete-plus-append.

## LightMem: online vs offline separation (ACL 2026 long)

- Memory work is modularized into retrieval, writing, and long-term
  consolidation, driven by small language models under bounded compute.
- Online processing stays cheap and incremental; offline consolidation runs
  separately and does the deeper reorganization.

Takeaway: this is exactly the sync/tidy split. Sync is the online layer:
cheap, deterministic, frequent. Tidy is the offline layer: model-heavy,
periodic, whole-store.

## Mem0: staged pipeline

- Two phases: extraction, then consolidation.
- Retrieval uses a vector store; conflict resolution weighs recency and
  importance.

Takeaway: keep extraction and consolidation as separate stages with separate
review gates. Resolve conflicts by recency, but confirm the newer fact is a
replacement and not a different claim (mtime wins only on identical claims;
the bridge already snapshots losers before overwriting).

## Semantic dedup: literal is not enough

- Dedup-by-name or exact-text misses facts that say the same thing in
  different words (openclaw-omg issue #99).
- Cluster-first reflection: group similar facts into clusters, then reflect
  over each cluster once. This finds duplicates and improves the merged
  quality in a single pass.
- Run dedup as a periodic pass, not during ingest (openclaw-omg #58): ingest
  stays fast and deterministic; a separate pass merges.

Practical thresholds used by tidy:

| Signal | Threshold | Meaning |
| --- | --- | --- |
| Jaccard on normalized word sets | >= 0.72 | strong duplicate signal (tidy-candidates default) |
| Jaccard | 0.60-0.72 | review list: related, needs model judgment |
| Embedding cosine on statement body | >= ~0.85 | semantically identical, different wording (no vector index bundled; optional) |

A merge is only valid when one statement entails the other. Shared vocabulary
alone is not evidence of a duplicate.

## Retention heuristics

Keep: rules, preferences, procedures, and decisions that outlive the task.

Drop: execution state, task/plan/transcript detail, incidents without a rule,
and facts that restate checked-in guidance.

When in doubt, keep the rule and drop the incident; when a fact has no
replacement and no backup, keep it.

## Sources

- A-MEM: arXiv 2502.12110, NeurIPS 2025.
- LightMem: ACL 2026 long, aclanthology.org/2026.acl-long.588.
- openclaw-omg issue #99 (semantic dedup, cluster-first reflection) and #58
  (dedup as a periodic background pass).
- Mem0 documentation (two-phase extraction/consolidation pipeline).
