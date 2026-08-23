---
name: security-scan
description: Run and adjudicate workspace dependency vulnerability and supply-chain scans with append-only agent verdicts. Use for CVE, GHSA, malicious-package, dependency-audit, or independent security finding reviews; prefer Codex and retain Claude as a secondary reviewer.
disable-model-invocation: true
---

# Dependency security scan

This plugin owns deterministic dependency and IOC collection. Use the official
Codex Security plugin for semantic repository/diff discovery, threat modeling,
validation, or attack-path analysis. A dependency finding collected here may be
passed to Codex Security's triage workflow; keep this plugin's append-only
verdict log as the cross-run audit record.

The workflow separates expensive immutable collection from repeatable analysis and rendering.

```text
security-scan run
  -> raw and normalized findings
security-report pending / analyze
  -> append-only independent verdicts
security-report render
  -> CVE and supply-chain reports
```

## 1. Resolve scope and scan

Discovery is read-only:

Resolve this plugin's `scripts/` directory from the selected skill path, then
invoke the bundled wrappers. They automatically enter the plugin's locked Nix
shell when `uv` or scanner executables are absent, so the same commands work
from an installed plugin cache. In a source checkout the discovery command is:

```bash
${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}/scripts/security-scan discover
```

Scanning may take several minutes and update vulnerability databases. If the
user explicitly named the project scope and asked to scan it, proceed without a
redundant confirmation. Ask only when scope is ambiguous or `--all-projects`
would materially expand the request.

```bash
${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}/scripts/security-scan run --projects="$CSV"
```

Use `--all-projects` only when the user requested harness-wide coverage. Never
edit files under the resulting `raw/` directory.

## 2. Review pending findings

```bash
${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}/scripts/security-report pending \
  --scan-dir="$SCAN_DIR" --severity=HIGH --format=json
```

For each finding, inspect the lockfile, reachable code path, package source, advisory, fixed versions, and scanner agreement. Browsing authoritative advisory and package sources is required for time-sensitive security claims.

Use fresh-context Codex subagents by default for independent high-impact
reviews. Claude may provide a secondary compatibility review. Give each
reviewer the normalized finding and relevant repository paths; do not seed it
with another reviewer's verdict.

## 3. Append a verdict

Record the actual runtime and model. Use `codex` by default or `claude-code`
when the secondary reviewer actually runs Claude. Do not falsify or hard-code a
model identity.

```bash
${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}/scripts/security-report analyze \
  --scan-dir="$SCAN_DIR" --cluster-id="$CLUSTER" \
  --project="$PROJECT" --package="$PACKAGE" --version="$VERSION" \
  --agent-runtime="$RUNTIME" --agent-model="$MODEL" \
  --verdict="$VERDICT" --fix="$FIX" --notes="$EVIDENCE"
```

Allowed verdicts are `real`, `false-positive`, `fixed`, `accepted-risk`, and `superseded`. The confirmations log is append-only. Correct a prior row by appending a superseding row and then a new verdict; never delete history.

## 4. Render after a coherent review batch

```bash
${CLAUDE_PLUGIN_ROOT:-${CODEX_PLUGIN_ROOT:-.}}/scripts/security-report render --scan-dir="$SCAN_DIR"
```

`REPORT.md` covers vulnerability advisories; `XRAY.md` and `iocs/` cover supply-chain campaigns. Conflicting independent verdicts must remain visible for operator adjudication.

This skill does not authorize dependency upgrades, service isolation, credential rotation, pushes, or deployment. Propose remediation in incident-safe order and execute it only when requested.
