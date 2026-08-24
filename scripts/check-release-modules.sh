#!/usr/bin/env bash
set -euo pipefail

root=$(git rev-parse --show-toplevel)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin" "$work/repo/.github/workflows"
git -C "$work/repo" init -q
touch "$work/repo/.github/workflows/publish-npm.yml"
touch "$work/repo/.github/workflows/publish-pypi.yml"
touch "$work/repo/.github/workflows/publish-cargo.yml"

cat >"$work/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[[ $* == 'repo view --json nameWithOwner --jq .nameWithOwner' ]]
printf 'example/bridge\n'
EOF
cat >"$work/bin/npm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ $* == 'trust github --help' ]]; then
    exit "${NPM_TRUST_HELP_STATUS:-0}"
fi
printf '%s\n' "$*" >"$LASSO_RELEASE_RECORD"
EOF
cat >"$work/bin/npx" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >"$LASSO_RELEASE_RECORD"
EOF
cat >"$work/bin/xdg-open" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$LASSO_OPEN_RECORD"
EOF
chmod +x "$work/bin/gh" "$work/bin/npm" "$work/bin/npx" "$work/bin/xdg-open"

export PATH="$work/bin:$PATH"
export LASSO_RELEASE_RECORD="$work/record"
export LASSO_OPEN_RECORD="$work/open-record"
cd "$work/repo"

npm_helper="$root/modules/npm-release/plugin/scripts/setup-trusted-publisher"
pypi_helper="$root/modules/pypi-release/plugin/scripts/setup-trusted-publisher"
cargo_helper="$root/modules/cargo-release/plugin/scripts/setup-trusted-publisher"

"$npm_helper" bridge-egress
grep -Fxq 'trust github bridge-egress --file publish-npm.yml --repo example/bridge --allow-publish --yes' "$LASSO_RELEASE_RECORD"

NPM_TRUST_HELP_STATUS=1 "$npm_helper" bridge-egress --repo example/other
grep -Fxq -- '--yes npm@12.0.2 trust github bridge-egress --file publish-npm.yml --repo example/other --allow-publish --yes' "$LASSO_RELEASE_RECORD"

before=$(sha256sum "$LASSO_RELEASE_RECORD")
printed=$($npm_helper bridge-egress --print)
grep -Fq 'npm trust github bridge-egress' <<<"$printed"
test "$before" = "$(sha256sum "$LASSO_RELEASE_RECORD")"

pypi_fields=$($pypi_helper bridge-egress --pending --print)
grep -Fxq 'Project: bridge-egress' <<<"$pypi_fields"
grep -Fxq 'Owner: example' <<<"$pypi_fields"
grep -Fxq 'Workflow: publish-pypi.yml' <<<"$pypi_fields"
grep -Fxq 'URL: https://pypi.org/manage/account/publishing/' <<<"$pypi_fields"

"$pypi_helper" bridge-egress --environment release >/dev/null
grep -Fxq 'https://pypi.org/manage/project/bridge-egress/settings/publishing/' "$LASSO_OPEN_RECORD"

rm -f "$LASSO_OPEN_RECORD"
cargo_fields=$($cargo_helper bridge-common bridge-egress --print)
grep -Fxq 'Crate: bridge-common' <<<"$cargo_fields"
grep -Fxq 'Crate: bridge-egress' <<<"$cargo_fields"
grep -Fxq 'Workflow: publish-cargo.yml' <<<"$cargo_fields"

"$cargo_helper" bridge-common bridge-egress --environment release >/dev/null
grep -Fxq 'https://crates.io/crates/bridge-common/settings/new-trusted-publisher' "$LASSO_OPEN_RECORD"
grep -Fxq 'https://crates.io/crates/bridge-egress/settings/new-trusted-publisher' "$LASSO_OPEN_RECORD"

if "$npm_helper" 'Bad Package' >/dev/null 2>&1; then
	printf 'invalid npm package unexpectedly accepted\n' >&2
	exit 1
fi
if "$pypi_helper" 'Bad Project!' >/dev/null 2>&1; then
	printf 'invalid PyPI project unexpectedly accepted\n' >&2
	exit 1
fi
if "$cargo_helper" 'bad.crate' >/dev/null 2>&1; then
	printf 'invalid crate unexpectedly accepted\n' >&2
	exit 1
fi

for template in \
	"$root/modules/npm-release/plugin/assets/package-page.md" \
	"$root/modules/pypi-release/plugin/assets/package-page.md" \
	"$root/modules/cargo-release/plugin/assets/package-page.md"; do
	test -s "$template"
	grep -Fq '[FIRST_USEFUL_COMMAND]' "$template"
done

printf 'release modules acceptance OK\n'
