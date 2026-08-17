# CLI conventions

Lasso ships one Go/cobra binary, `lasso`, built into `./bin` by
`just build`. Optional plugin CLIs are exposed only through their owning plugin
scripts and never become harness subcommands.

`lasso version --format=json` is the host/plugin compatibility contract.
It reports the CLI version, integer API version, and source revision injected by
the owning build. Plugins fail closed on an unsupported API instead of probing
legacy commands opportunistically.

## Shape

- Use full top-level nouns: `project`, `work-item`.
- Use a positional argument for the command's single primary resource:
  `project path <name>`, `work-item show <id>`, `work-item rm <id>`.
- Use named flags for secondary scope or behavior: `--project`,
  `--format`, `--force`, `--yes`.
- Never accept two ambiguous positional identifiers in the same command.
- Destructive commands require an explicit confirmation flag or interactive
  confirmation and must state what they do not remove.

Keep command help authoritative. Include the ownership boundary in help when a
reasonable caller might assume broader behavior; for example, work-item removal
does not mutate a checkout or Codex worktree.

## Structured output

Commands that emit collections or records support:

| Value | Use |
|---|---|
| `table` | default human view |
| `json` | stable programmatic interchange |
| `yaml` | readable record/diff output |

Structured stdout contains only the payload. Progress and warnings go to
stderr. Prefer JSON in agent and script calls. Do not add custom encodings when
standard JSON plus an optional output schema is sufficient.

## Implementation

| Path | Role |
|---|---|
| `cmd/lasso/main.go` | thin process entry point |
| `internal/cli/<command>.go` | cobra wiring and presentation |
| `internal/config/` | strict TOML project registry |
| `internal/gitx/` | small Git command adapter |
| `internal/worktree/` | state-free canonical task-worktree lifecycle |
| `internal/workitems/` | versioned durable coordination metadata |
| `internal/project/` | safe multi-file registration mutations |

Keep mutation and validation logic out of cobra handlers. Avoid runtime
reflection and dual schema ownership. Prefer standard library formats with
strict decoding and focused tests.

## `justfile`

Recipes are discoverable entry points, not a second implementation language.
Use them to call Go, `uv`, and linters. Move branching or reusable logic into a
tested package or project-owned script.
