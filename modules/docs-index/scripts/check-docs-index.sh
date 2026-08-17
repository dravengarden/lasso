#!/usr/bin/env bash
# Validate docs/INDEX.md coverage for the current workspace (or kit).
set -euo pipefail

root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$root"

if [[ ! -f docs/INDEX.md ]]; then
    echo "missing docs/INDEX.md" >&2
    exit 1
fi

first=$(sed -n '1p' docs/INDEX.md)
[[ $first == '---' ]] || {
    echo "docs/INDEX.md: missing frontmatter" >&2
    exit 1
}

if ! grep -q '^type: docs_index$' docs/INDEX.md; then
    echo "docs/INDEX.md: frontmatter must include type: docs_index" >&2
    exit 1
fi

fail=0
while IFS= read -r -d '' f; do
    rel=${f#./}
    [[ $rel == docs/INDEX.md ]] && continue
    # Allow any path form that ends with the relative file.
    if ! grep -qE "\\]\\(\\./${rel#docs/}|\\]\\(${rel}|${rel}\\)" docs/INDEX.md \
        && ! grep -Fq "${rel}" docs/INDEX.md \
        && ! grep -Fq "./${rel#docs/}" docs/INDEX.md \
        && ! grep -Fq "${rel#docs/}" docs/INDEX.md; then
        echo "docs/INDEX.md: unindexed page $rel" >&2
        fail=1
    fi
done < <(find docs -type f -name '*.md' -print0 | sort -z)

exit "$fail"
