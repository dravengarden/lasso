# Commit messages

## Per project

Each project repo may have its own commit conventions. When working inside a checkout,
defer to the project's own `AGENTS.md` if it specifies something.
Otherwise, fall back to the rules below.

## Default rules

- Subject line ≤ 72 chars, imperative mood ("add foo", not "added foo")
- Body explains _why_, not _what_. Diff shows what.

## Harness commits

Commits to the harness git itself (this outer repo) follow a `<area>: <verb>` form:

- `registry: decode strict per-project TOML entries`
- `checkout: create ordinary clone during setup`
- `docs: clarify Codex worktree ownership`
- `projects: register new-service`
