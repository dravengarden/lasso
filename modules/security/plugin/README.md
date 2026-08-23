# Lasso Security

An optional Codex-first plugin for cross-project dependency and supply-chain
scans. Claude Code can consume the same workflow through its thin plugin
manifest and remains a secondary reviewer. The plugin is deliberately outside
the Columbus harness core.

OpenAI's `codex-security` plugin owns semantic source/diff scanning, threat
modeling, validation, and attack-path analysis. Lasso Security owns the
residual deterministic layer: repeatable multi-feed dependency collection
across registered projects, scanner-agreement evidence, local IOC matching, and
append-only adjudication history. Do not grow a second semantic source scanner
here; pass normalized findings to Codex Security when deeper code-level triage
is needed.

The bundled skill owns review sequencing; `scripts/security-scan` and
`scripts/security-report` are stable wrappers around the Python package under
`scripts/agent_tools/`. The wrappers work from either a source checkout or an
installed plugin cache: when their pinned tools are absent, they re-enter this
plugin's locked Nix development shell automatically. Nix is therefore the only
host prerequisite for an installed copy.

```bash
cd plugins/lasso-security
nix develop -c just verify
```

Python dependencies belong in `scripts/agent_tools/pyproject.toml`. Tests are
co-located as `*_test.py`; Ruff, ty, and pytest define the plugin quality gate.
