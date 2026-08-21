# Code style + quality gate

Cross-language rules for any code touched in this monorepo. A closer
project-owned `AGENTS.md` or language-specific convention under this directory
wins when it is more specific.

Language-specific defaults live under this directory, including the
[first-party Rust development standard](rust/rust.md) and
[Julia code guidelines](julia/julia.md).

## Language — English only

All code identifiers, comments, docstrings, and committed documentation
**must be English**. The only exception is user-facing strings shown to
end users (UI labels, error messages).

| Surface                                       | Language                   |
| --------------------------------------------- | -------------------------- |
| Variable / function / class / file names      | English                    |
| Inline comments, docstrings, JSDoc, TSDoc     | English                    |
| Committed `*.md`, ADRs, commit messages       | English                    |
| User-facing UI strings, end-user error copy   | Target language (any)      |

```python
# ✅ English identifiers and comments
def calculate_average_response_time(requests: list[Request]) -> float:
    """Calculate the average response time across all requests."""
    total_time = sum(r.duration_ms for r in requests)
    return total_time / len(requests)

# ❌ Chinese identifiers or comments
def 计算平均响应时间(请求列表: list[Request]) -> float:
    """计算所有请求的平均响应时间"""
    总时间 = sum(r.duration_ms for r in 请求列表)  # 求和
    return 总时间 / len(请求列表)
```

Why: code is read more often than written. English unblocks global
collaboration, third-party library integration, AI assistance, and
long-term maintenance. Enforcement is by pre-commit hook
(non-ASCII identifier check), code review, and language-specific
linters.

## Quality gate — before declaring a task done

Run `just verify` and clear all diagnostics before saying you are
finished. It runs the strict checks across the harness:

- **Go** via `go vet` and `go test`
- **Bash** via `shellcheck` with `.shellcheckrc` config
- **Nix and Markdown** through the root lint recipes

Workflow:

- Use `just fmt` first when violations are mechanical.
- Use `just lint` to run Go, shell, Markdown, and Nix checks without fixing.
- Anything still red must be fixed by hand; do not suppress a diagnostic
  without a documented reason.
- `just test` runs Go tests; `just verify` runs format + lint + test plus the
  independently packaged plugin gate.

## Codex quality integration

`just verify` is the canonical quality gate and must run before Codex reports a
code change complete. A repository or user-level Codex hook may invoke
`nix develop -c just verify-changed` as an optimization, but the hook is not a
substitute for the explicit task verification evidence.

Carve-outs:

- `cwd` inside `projects/` — each project has its own conventions
- `cwd` outside the Lasso workspace root entirely
- `git commit -m "$(cat <<'EOF' …)"` heredocs (commit-message text
  isn't shell code)
