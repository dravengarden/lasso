# Lasso product guidance

This repository is the **Lasso product (kit)**: CLI, core plugin, module catalog,
and workspace templates. It is not a user fleet instance (those are created with
`lasso init`).

## Development

```bash
nix develop   # optional
just build
just test
just verify
```

Binaries land in `./bin`. Put `./bin` on `PATH`. Set `LASSO_KIT_ROOT=$PWD` when
testing module install from another directory.

## Ownership

- Go CLI: `cmd/lasso`, `internal/`
- Installable workflows: `plugins/lasso-core`
- Optional packs: `modules/` + `modules/catalog.toml`
- Init template: `templates/workspace`
- Meta-skills (also shipped in core plugin): `skills/lasso-*`

## Agent runtimes

Codex is the baseline. Claude and Grok follow shared skills via dual plugin
manifests under each plugin. Do not fork skill bodies per runtime.

## Quality gate

`just verify` before commit: format, tests, skill frontmatter, docs index.
