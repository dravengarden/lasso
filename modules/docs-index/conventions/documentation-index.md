# Documentation index convention

Every Lasso-owned documentation tree has one canonical navigation entry:
`docs/INDEX.md`. The index is a review surface for both people and agents; it
is not a second copy of the documentation.

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

1. a short scope statement;
2. a reading order for a new reader;
3. grouped links for the categories that exist;
4. a link to every Markdown page below the `docs/` root, excluding the index.

Use relative links. Adding, moving, or removing a Markdown page requires
updating the nearest canonical index in the same change.

## Gate

```bash
bash scripts/check-docs-index.sh
```

Or, when the module scripts are installed under the workspace:

```bash
bash .lasso/modules/docs-index/scripts/check-docs-index.sh
```
