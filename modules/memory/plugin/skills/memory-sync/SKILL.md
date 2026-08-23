---
name: memory-sync
description: Codex-only writer that moves durable facts between ordinary and DeepSeek Codex homes; Claude Code and Grok may use only its read-only stale-path scan. Use in Codex when one Codex home knows something another does not, or when a follower memory store looks stale.
disable-model-invocation: true
---

# Cross-runtime memory sync

Sync moves durable facts between structurally compatible Codex homes: add
(copy or extract), modify (report differences for review), and retire (additive
notes). Claude Code and Grok retain separate native stores. This plugin only
scans those follower stores for stale path references; it never extracts,
writes, bridges, or rewrites their content or indexes. Grok's audited surface
includes global, workspace, and session Markdown below `~/.grok/memory`.

Codex owns every mutating command in this workflow. If the requested outcome
needs `extract`, `notes`, `memory-bridge --write`, or any direct change below a
Codex memory home, stop and ask the user to continue in Codex.

Tools:

- `memory-sync scan|stale|extract|notes` — bundled wrapper; enters the
  plugin's locked Nix shell on its own
- `memory-bridge [--write] [--propagate-updates]` — plugin script, dry-run by
  default

Resolve this plugin's `scripts/` directory from the selected skill path and
run the bundled wrappers. Command syntax lives in `--help`.

## 1. Start with the free checks

Run `scan` and `stale` first. Both are deterministic, need no network, and
answer most requests on their own.

`stale` is the one signal a model cannot produce, because it requires touching
the filesystem. It scans Claude and Grok stores read-only in addition to
Codex staging. Treat every finding as a review item, never as an instruction to
delete. A fact naming a retired checkout is usually worth rewriting rather than
removing.

## 2. Add: extract and notes

Run `extract` when one Codex home has learned something another has not.
It requires a host DeepSeek gateway on loopback; the plugin holds no
credential of its own.

Pass the repository and machine guidance files that are already in force. The
most valuable thing the model does is drop statements those files already
mandate, and it cannot do that without seeing them. Start with a small
`--limit` on a first run, read the proposals, then widen. Cost is a few cents
for a full pass, so the limit is about your review capacity, not about spend.

`notes` writes into Codex's `extensions/ad_hoc/notes/`, which is the only
supported entry point. Codex compiles `MEMORY.md` and `memory_summary.md`
from rollout summaries and those notes, so writing to either artifact
directly is lost at the next consolidation.

`memory-bridge --write` incrementally copies missing notes between distinct
Codex homes. Rollout summaries are bootstrap-only: they are copied into a
store that has never consolidated (no compiled `MEMORY.md`) and are never
re-copied once consolidation exists, because consolidation consumes and prunes
them. The bridge never reads or writes Claude or Grok facts or their
runtime-owned `MEMORY.md` indexes. A fact is refused when its destination is the
resolved store it came from, and
that refusal is not configurable: without it the two directions form a loop.
Because the check compares stores rather than runtimes, one Codex home may
sync into another. The DeepSeek provider keeps its own store under
`~/.local/state/cowboy/providers/codex-deepseek/codex-home`, which starts
empty while the ordinary one holds everything learned so far.

## 3. Modify: propagate updates

`memory-bridge --propagate-updates` reports same-name/different-content
differences between Codex homes. Codex notes and rollout summaries are never overwritten: the bridge reports
same-name/different-content differences instead. Resolving those is a tidy
operation (merge into a new note plus a retirement note), not a clobber.

## 4. Delete and retire

- Claude and Grok stores: this plugin reports stale paths only. Use the owning
  runtime's explicit memory controls for any rewrite, archive, or index repair.
- Codex: notes are never deleted or replaced, per the extension's own
  instructions. To retire a fact, add a note stating that it no longer holds.
- Stale references: `stale` reports them; rewrite or retire after review.

## 5. Review before anything reaches a store

`extract` writes nothing without `--write`, and even then only into the
staging directory. Staging is the review surface; a live memory store is
never written by this plugin.

One check already ran in code: candidates pinning a port, address, revision,
version, source symbol, or store path were dropped, because the model kept
them even when the prompt named them as grounds for rejection.

The guidance check did not work in code and is your job: in measured runs the
model never marked a candidate as already covered, including one whose
mandating paragraph sat directly beside it in the request. Compare the staged
facts against `AGENTS.md` and machine guidance yourself.

Read the staged facts and reject any that:

- restate a rule already in `AGENTS.md` or machine guidance
- cannot be understood without the original task context
- describe execution state such as a task, goal, plan, or transcript
- record an incident rather than the rule the incident taught

Use `--keep-volatile` only when the pinned detail is the point and you intend
to re-check it, such as a short-lived migration note.

Check the reported conflicts. A conflict means two task groups disagreed and
the later one was kept. Confirm that choice is right before the fact ships;
file order is a proxy for recency, not proof of it.

## 6. Stop conditions

Stop and ask rather than proceeding when:

- the gateway is unreachable; do not fall back to a direct provider call or
  introduce a credential
- a proposal would encode a person's private information rather than a working
  preference
- staleness findings suggest a whole store is keyed to an obsolete path; that
  is a fragmentation problem to fix at the source, not one to paper over by
  copying facts around
