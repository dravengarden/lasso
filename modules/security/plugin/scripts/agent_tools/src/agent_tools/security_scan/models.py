"""Pydantic data models shared across the security-scan skill.

Convention (memory: `feedback_python_data_models.md`): every structured
record crosses JSON boundaries at some point — scanner outputs, the
normalized line format, the merged unified view, the append-only
confirmation trail. Each lives as a `pydantic.BaseModel` so we get
runtime validation, auto-coercion, and consistent error messages.

Module-level invariants:
- All models opt-in to `extra='allow'` so scanner-specific fields we
  don't normalize over (e.g. trivy's `CVSS`, grype's `matchDetails`)
  pass through without crashing the parser.
- Severity is a `Literal[...]` union so misspellings fail at validation
  time instead of leaking through to the report.
- `Confirmation.verdict` is similarly closed-set.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING, Annotated, Literal, Self

from pydantic import BaseModel, ConfigDict, Field

if TYPE_CHECKING:
    from collections.abc import Iterable

# ─── shared literal types ────────────────────────────────────────────

Severity = Literal["CRITICAL", "HIGH", "MODERATE", "LOW", "UNKNOWN"]
# `safety` remains readable for historical scan bundles, but is no longer an
# active scanner because its unauthenticated legacy database and command are
# deprecated.
ScannerName = Literal["osv-scanner", "pip-audit", "trivy", "grype", "safety"]
Verdict = Literal["real", "false-positive", "fixed", "accepted-risk", "superseded"]
EcosystemHint = Literal["PyPI", "npm", "Go", "Maven", "RubyGems", "Cargo", "?"]

SEV_RANK: dict[str, int] = {
    "CRITICAL": 0,
    "HIGH": 1,
    "MODERATE": 2,
    "LOW": 3,
    "UNKNOWN": 4,
}

# Tuple of all scanner names — useful for iteration in report rendering.
SCANNERS: tuple[ScannerName, ...] = (
    "osv-scanner",
    "pip-audit",
    "trivy",
    "grype",
)

# Tuple of all verdicts in display order.
VERDICTS: tuple[Verdict, ...] = (
    "real",
    "false-positive",
    "fixed",
    "accepted-risk",
    "superseded",
)


# ─── discovery models ────────────────────────────────────────────────


class DepFiles(BaseModel):
    """Per-project inventory of dep-file paths (relative to worktree)."""

    requirements_txt: list[str] = Field(default_factory=list)
    pyproject_toml: list[str] = Field(default_factory=list)
    uv_lock: list[str] = Field(default_factory=list)
    package_lock_json: list[str] = Field(default_factory=list)
    package_json: list[str] = Field(default_factory=list)
    cargo_lock: list[str] = Field(default_factory=list)
    go_sum: list[str] = Field(default_factory=list)

    def total(self) -> int:
        """Return the total number of dep-file paths across all categories."""
        return (
            len(self.requirements_txt)
            + len(self.pyproject_toml)
            + len(self.uv_lock)
            + len(self.package_lock_json)
            + len(self.package_json)
            + len(self.cargo_lock)
            + len(self.go_sum)
        )

    def has_python(self) -> bool:
        """Return True if any Python dependency manifest was found."""
        return bool(self.requirements_txt or self.pyproject_toml or self.uv_lock)

    def has_npm(self) -> bool:
        """Return True if an npm lockfile was found."""
        return bool(self.package_lock_json)

    def has_rust(self) -> bool:
        """Return True if a Cargo lockfile was found."""
        return bool(self.cargo_lock)

    def has_go(self) -> bool:
        """Return True if a Go checksum file was found."""
        return bool(self.go_sum)


class ProjectEntry(BaseModel):
    """One project's full scan-readiness record."""

    project: str
    registered: bool
    branch: str | None = None
    present: bool = False
    worktree: str = ""
    languages: list[str] = Field(default_factory=list)
    deps: DepFiles = Field(default_factory=DepFiles)
    scannable: bool = False
    error: str = ""


# ─── scanner-emitted finding ────────────────────────────────────────


class Finding(BaseModel):
    """One row in normalized/findings.jsonl — common-schema scanner output."""

    model_config = ConfigDict(extra="allow")

    scanner: ScannerName
    project: str
    source_file: str = ""
    package: str
    ecosystem: str = "?"
    version: str
    primary_id: str = "UNKNOWN"
    all_ids: list[str] = Field(default_factory=list)
    severity: Severity = "UNKNOWN"
    summary: str = ""


# ─── unified post-merge row ─────────────────────────────────────────


class UnifiedRow(BaseModel):
    """One deduped (project, package, version, cluster) entry."""

    model_config = ConfigDict(extra="allow")

    project: str
    package: str
    version: str
    ecosystem: str = "?"
    cluster_id: str
    all_ids: list[str] = Field(default_factory=list)
    severity: Severity = "UNKNOWN"
    scanners: list[ScannerName] = Field(default_factory=list)
    scanner_count: int = 0
    source_files: list[str] = Field(default_factory=list)
    summary: str = ""

    def key(self) -> tuple[str, str, str, str]:
        """Return the cluster/project/package/version dedup key."""
        return (self.cluster_id, self.project, self.package, self.version)


# ─── agent confirmation row ─────────────────────────────────────────


class Confirmation(BaseModel):
    """One row in confirmations.jsonl — agent verdict on a finding.

    Agent identity is `(agent_runtime, agent_model)` — examples:
      - `("codex", "current-model-id")`
      - `("claude-code", "current-model-id")`
      - `("human", "reviewer-name")` — escape hatch for manual review

    Display string is `f"{agent_runtime} {agent_model}"`. Multiple agents
    can append confirmations for the same finding; the renderer marks
    them as a CONFLICT when verdicts diverge.
    """

    model_config = ConfigDict(extra="allow")

    cluster_id: str
    project: str
    package: str
    version: str
    agent_runtime: str
    agent_model: str
    confirmed_at: str
    verdict: Verdict
    fix: str = ""
    notes: str = ""
    supersedes: str = ""

    def key(self) -> tuple[str, str, str, str]:
        """Return the cluster/project/package/version dedup key."""
        return (self.cluster_id, self.project, self.package, self.version)

    def agent_id(self) -> str:
        """Human-readable agent identity for display in reports."""
        return f"{self.agent_runtime} {self.agent_model}".strip()


# ─── tools manifest ─────────────────────────────────────────────────


class ToolEntry(BaseModel):
    """One scanner's version + vuln-db description for the report header."""

    version: str = "(unknown)"
    vuln_db: str = ""
    ecosystem: str = ""


class ToolsManifest(BaseModel):
    """`tools.json` — what scanners ran, with what data sources, when."""

    scanned_at: str = "(unknown)"
    harness_root: str = "(unknown)"
    tools: dict[str, ToolEntry] = Field(default_factory=dict)
    projects_scanned: list[str] = Field(default_factory=list)
    project_scanners: dict[str, list[ScannerName]] = Field(default_factory=dict)

    def scanners_for_project(self, project: str) -> list[str]:
        """Return verified scanners for one project, with old-bundle fallback."""
        if project in self.project_scanners:
            return list(self.project_scanners[project])
        if self.tools:
            return list(self.tools)
        return list(SCANNERS)

    def scanner_total_for(self, project: str) -> int:
        """Return the truthful agreement denominator for one project."""
        return len(self.scanners_for_project(project))


# ─── stats / summary records ────────────────────────────────────────


class NormalizeStats(BaseModel):
    """Returned by `normalize.normalize()`; the orchestrator prints it."""

    per_scanner: dict[str, int] = Field(default_factory=dict)
    total: int = 0
    output_path: Path


class MergeStats(BaseModel):
    """Returned by `merge.merge()`."""

    raw_count: int = 0
    unified_count: int = 0
    severity_counts: dict[str, int] = Field(default_factory=dict)
    coverage_counts: dict[int, int] = Field(default_factory=dict)
    unified_path: Path
    coverage_path: Path
    per_project_path: Path


class RenderResult(BaseModel):
    """Returned by `render.render()`.

    Two report files are produced:
    - `md_path` / `html_path`         — main CVE/GHSA report (REPORT.*)
    - `xray_md_path` / `xray_html_path` — supply-chain report (XRAY.*)

    Plus optional per-campaign IOC pages under `iocs/`.
    """

    md_path: Path
    html_path: Path
    xray_md_path: Path
    xray_html_path: Path
    unified_count: int = 0
    confirmations_count: int = 0
    conflicts_count: int = 0
    ioc_match_count: int = 0


class SummaryStats(BaseModel):
    """Computed by render.py for the report header."""

    total: int = 0
    sev_counts: dict[str, int] = Field(default_factory=dict)
    actionable: int = 0
    verdict_counts: dict[str, int] = Field(default_factory=dict)
    confirmed_total: int = 0
    # Preformatted lines for Jinja — keeps the template free of whitespace gymnastics.
    sev_summary: str = ""
    verdict_summary: str = ""


# ─── orchestrator-internal records ──────────────────────────────────


class IocPackage(BaseModel):
    """One package + version-list entry inside an IocCampaign."""

    model_config = ConfigDict(extra="allow")

    name: str
    versions: list[str] = Field(default_factory=list)
    xray_id: str = ""


class IocNetwork(BaseModel):
    """Network IOCs (IPs and domains) associated with a campaign."""

    ips: list[str] = Field(default_factory=list)
    domains: list[str] = Field(default_factory=list)


class IocCampaign(BaseModel):
    """One supply-chain campaign feed (e.g. Shai-Hulud).

    Lives in `data/iocs/*.json`. Each campaign carries: malicious
    package+version tuples per ecosystem, network IOCs, file hashes,
    string indicators (commit-author email patterns, dead-drop repo
    description, etc.).
    """

    model_config = ConfigDict(extra="allow")

    campaign: str
    attribution: str = ""
    source: str = ""
    published_at: str = ""
    ingested_at: str = ""
    category: str = "supply-chain-malicious-package"
    summary: str = ""
    remediation_note: str = ""
    ecosystems: dict[str, list[IocPackage]] = Field(default_factory=dict)
    network_iocs: IocNetwork = Field(default_factory=IocNetwork)
    file_hashes_sha256: dict[str, list[str] | str] = Field(default_factory=dict)
    campaign_strings: list[str] = Field(default_factory=list)
    github_indicators: dict[str, list[str] | str] = Field(default_factory=dict)
    persistence_artifacts: dict[str, list[str]] = Field(default_factory=dict)


MatchKind = Literal["exact", "name-only"]


class IocMatch(BaseModel):
    """One concrete hit against a campaign's package list.

    Two `match_kind` flavors:

    - `exact`     — same `(name, version)` as the campaign listed: BAD,
                    you're running a confirmed malicious release.
    - `name-only` — same package name but a different version. Useful as
                    a "you're using this lib at a safe version today, but
                    a future upgrade could pull you into the bad range —
                    pin tight + audit upgrades" warning. `version` is the
                    version we actually have; `flagged_versions` lists
                    what the campaign called malicious.
    """

    campaign: str
    source: str
    project: str
    package: str
    version: str
    ecosystem: str
    xray_id: str = ""
    where: str = ""
    detected_by_scanner: bool = False
    scanner_clusters: list[str] = Field(default_factory=list)
    match_kind: MatchKind = "exact"
    flagged_versions: list[str] = Field(default_factory=list)

    def key(self) -> tuple[str, str, str, str, str]:
        """Return the campaign/project/package/version/ecosystem dedup key."""
        return (self.campaign, self.project, self.package, self.version, self.ecosystem)


class ScannerVersions(BaseModel):
    """Captured at scan time; folded into ToolsManifest."""

    osv_scanner: str = "(unknown)"
    pip_audit: str = "(unknown)"
    trivy: str = "(unknown)"
    grype: str = "(unknown)"


class DepTarget(BaseModel):
    """One pip-audit per-file scan target."""

    tag: Annotated[str, Field(description="tag used to name the output file")]
    path: Path


# ─── helpers ────────────────────────────────────────────────────────


def load_jsonl[T: BaseModel](path: Path, model: type[T]) -> list[T]:
    """Read one model instance per non-blank line. Returns [] if missing."""
    if not path.exists():
        return []
    with path.open() as fp:
        return [model.model_validate_json(line) for line in fp if line.strip()]


def write_jsonl(path: Path, rows: Iterable[BaseModel]) -> None:
    """Write one JSON object per line. Truncates and overwrites."""
    with path.open("w") as fp:
        for r in rows:
            fp.write(r.model_dump_json() + "\n")


def append_jsonl(path: Path, row: BaseModel) -> None:
    """Append one JSON object as a new line. Creates the file if missing."""
    with path.open("a") as fp:
        fp.write(row.model_dump_json() + "\n")


def load_tools_manifest(path: Path) -> ToolsManifest:
    """Read `tools.json` if present; fall back to a default-valued manifest."""
    if not path.exists():
        return ToolsManifest()
    try:
        return ToolsManifest.model_validate_json(path.read_text())
    except json.JSONDecodeError, ValueError:
        return ToolsManifest()


# `Self` is exported for downstream users who chain validators — quiet
# the unused-symbol warning here.
_ = Self
