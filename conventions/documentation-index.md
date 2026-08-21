# Documentation index convention

Every Lasso-owned documentation tree has one canonical navigation entry:
`docs/INDEX.md`. The index is a review surface for both people and agents; it
is not a second copy of the documentation.

## Scope

This convention applies to:

- the Lasso product `docs/` tree;
- a workspace's owned `docs/` tree after `lasso init`;
- a registered project's `projects/<name>/docs/` tree when that checkout is
  present;
- optional instance docs trees (for example host or machine docs) when a
  workspace opts into the same INDEX contract.

Generated build output, dependency/vendor documentation, caches, and arbitrary
documentation directories nested inside a dependency are not Lasso-owned
documentation roots. Project documentation remains owned by the project; the
Lasso checker only enforces the shared shape and link coverage.

## Canonical shape

The file name is exactly `INDEX.md` and the file starts with:

```markdown
---
type: docs_index
description: Short description of the documentation tree.
---

# Documentation index
```

After the heading, every index should provide:

1. a short scope statement identifying the audience and the current boundary;
2. a reading order or orientation path for a new reader;
3. grouped links for architecture, contracts, operations, research, decisions,
   or the categories that actually exist in that project;
4. a link to every Markdown page below the `docs/` root, excluding the index
   itself.

Use relative links and descriptive labels. A link may point outside the docs
root when that is the documented ownership boundary, but every local target
must exist. Do not use an index to hide an unreviewed page: adding, moving, or
removing a Markdown page requires updating the nearest canonical index in the
same change.

Large trees may use category sections and nested index pages, but the root
index must still make the navigation path explicit. A category index is a
normal Markdown page and is linked from its parent index.

## Review contract

Before reviewing implementation work, an agent or person should be able to
start at `INDEX.md`, understand the current reading order, and reach every
page without guessing filenames. The index should distinguish normative
requirements, design proposals, operational runbooks, research evidence, and
historical records where those categories exist. It must not promote a draft,
shadow result, or historical note into an operational contract merely by
placing it earlier in the reading order.

`just docs-index-check` is the deterministic gate. It checks the canonical
filename and metadata, validates local links, and fails on an unindexed
Markdown page. `just architecture-check` includes the same gate, so every
Lasso core verification also checks documentation navigation.

## Codex and Claude boundary

Codex already reads repository guidance and can follow links in ordinary
Markdown. Lasso owns the residual cross-project contract: a stable index
shape and deterministic coverage check across registered checkouts. The
canonical artifact is the repository's `docs/INDEX.md`, and the checker is the
narrowest mechanical extension surface; it does not persist task, transcript,
memory, review, or worktree state.

Claude Code follows the same shared index files and `just docs-index-check`
command. No Claude-specific policy copy is permitted. If Codex later provides
the same cross-project index discovery and coverage guarantee, this Lasso
checker can be removed while retaining the project-owned `INDEX.md` files.
