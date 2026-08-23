"""Render REPORT.md and report.html from merged findings + confirmations.

Pure library module. The orchestrator (`scan.py` / `confirm.py`) imports
`render` and exposes the user-facing flag.

Templates live at `../../templates/report.{md,html}.j2`. Context passed
identically to both:

  today           : ISO date string
  summary         : SummaryStats
  tools_manifest  : ToolsManifest
  by_project      : OrderedDict[project_name, list[UnifiedRow]]   (sorted)
  confs_by_key    : dict[row_key_tuple, list[Confirmation]]
  audit_trail     : list[Confirmation]                            (chronological)
  helpers         : namespace with severity_class, effective_verdict,
                    fix_for, scanner_short, advisory_url, row_key,
                    project_sev_counts, project_actionable
"""

from __future__ import annotations

import datetime as dt
import re
from collections import OrderedDict, defaultdict
from pathlib import Path
from types import SimpleNamespace

from jinja2 import Environment, FileSystemLoader, StrictUndefined, select_autoescape

from .iocs import (
    load_campaigns,
    match_against_project_deps,
    match_against_unified,
)
from .models import (
    SCANNERS,
    SEV_RANK,
    Confirmation,
    IocCampaign,
    IocMatch,
    RenderResult,
    SummaryStats,
    ToolsManifest,
    UnifiedRow,
    load_jsonl,
    load_tools_manifest,
)

# ─── visual classifications (used by templates via `helpers`) ──────

SEV_CLASS: dict[str, str] = {
    "CRITICAL": "sev-crit",
    "HIGH": "sev-high",
    "MODERATE": "sev-mod",
    "LOW": "sev-low",
    "UNKNOWN": "sev-unk",
}
SCANNER_SHORT: dict[str, str] = {
    "osv-scanner": "osv",
    "pip-audit": "pip-a",
    # Historical scan bundles may still contain the retired Safety adapter.
    "safety": "safety",
    "trivy": "trivy",
    "grype": "grype",
}

RowKey = tuple[str, str, str, str]


# ─── confirmation grouping / aggregation ────────────────────────────


def _group_confirmations(confs: list[Confirmation]) -> dict[RowKey, list[Confirmation]]:
    out: dict[RowKey, list[Confirmation]] = defaultdict(list)
    for c in confs:
        out[c.key()].append(c)
    for group in out.values():
        group.sort(key=lambda c: c.confirmed_at)
    return out


def _live_confirmations(confs: list[Confirmation]) -> list[Confirmation]:
    """Drop `superseded` entries (and what they supersede)."""
    superseded_at: set[str] = {c.supersedes for c in confs if c.supersedes}
    return [c for c in confs if c.verdict != "superseded" and c.confirmed_at not in superseded_at]


def _verdicts_per_agent(confs: list[Confirmation]) -> dict[str, str]:
    """Map agent identity → most-recent live verdict from that agent.

    Each agent may have analyzed a finding multiple times; we keep their
    latest non-superseded verdict.
    """
    live = _live_confirmations(confs)
    out: dict[str, str] = {}
    for c in live:
        out[c.agent_id()] = c.verdict
    return out


def _effective_verdict(confs: list[Confirmation]) -> str:
    """Consensus verdict across agents, or empty string if no live verdicts.

    When agents disagree the literal string `"conflict"` is returned so
    callers / templates can highlight it. Use `_verdicts_per_agent` to
    pull the actual divergent verdicts.
    """
    per_agent = _verdicts_per_agent(confs)
    if not per_agent:
        return ""
    distinct = set(per_agent.values())
    if len(distinct) == 1:
        return next(iter(distinct))
    return "conflict"


def _is_actionable(row: UnifiedRow, confs: list[Confirmation]) -> bool:
    """True if the finding still needs a fix.

    Once any agent says false-positive/fixed/accepted-risk AND no other
    agent disagrees, we treat it as resolved. Conflicts stay actionable
    until adjudicated.
    """
    verdict = _effective_verdict(confs)
    if verdict in {"false-positive", "fixed", "accepted-risk"}:
        return False
    return row.severity in {"CRITICAL", "HIGH"}


def _has_conflict(confs: list[Confirmation]) -> bool:
    return _effective_verdict(confs) == "conflict"


SEV_ORDER: tuple[str, ...] = ("CRITICAL", "HIGH", "MODERATE", "LOW", "UNKNOWN")
VERDICT_ORDER: tuple[str, ...] = ("real", "false-positive", "fixed", "accepted-risk", "superseded")


def _summary(
    unified: list[UnifiedRow],
    confs_by_key: dict[RowKey, list[Confirmation]],
) -> SummaryStats:
    sev_counts: dict[str, int] = defaultdict(int)
    actionable = 0
    verdict_counts: dict[str, int] = defaultdict(int)
    for r in unified:
        sev_counts[r.severity] += 1
        confs = confs_by_key.get(r.key(), [])
        if _is_actionable(r, confs):
            actionable += 1
        v = _effective_verdict(confs)
        if v:
            verdict_counts[v] += 1
    sev_summary = "  ".join(f"{s}={sev_counts[s]}" for s in SEV_ORDER if sev_counts[s])
    verdict_summary = ", ".join(f"{v}={verdict_counts[v]}" for v in VERDICT_ORDER if verdict_counts[v])
    return SummaryStats(
        total=len(unified),
        sev_counts=dict(sev_counts),
        actionable=actionable,
        verdict_counts=dict(verdict_counts),
        confirmed_total=sum(verdict_counts.values()),
        sev_summary=sev_summary,
        verdict_summary=verdict_summary,
    )


def _project_sev_counts(rows: list[UnifiedRow]) -> dict[str, int]:
    out: dict[str, int] = defaultdict(int)
    for r in rows:
        out[r.severity] += 1
    return dict(out)


def _project_actionable(
    rows: list[UnifiedRow],
    confs_by_key: dict[RowKey, list[Confirmation]],
) -> int:
    return sum(1 for r in rows if _is_actionable(r, confs_by_key.get(r.key(), [])))


def _by_project(
    unified: list[UnifiedRow],
    confs_by_key: dict[RowKey, list[Confirmation]],
) -> OrderedDict[str, list[UnifiedRow]]:
    """Group findings by project, most-actionable first."""
    grouped: dict[str, list[UnifiedRow]] = defaultdict(list)
    for r in unified:
        grouped[r.project].append(r)
    sorted_projects = sorted(
        grouped.keys(),
        key=lambda p: (-_project_actionable(grouped[p], confs_by_key), p),
    )
    out: OrderedDict[str, list[UnifiedRow]] = OrderedDict()
    for p in sorted_projects:
        out[p] = sorted(
            grouped[p],
            key=lambda r: (SEV_RANK[r.severity], -r.scanner_count, r.package),
        )
    return out


def _fix_for(row: UnifiedRow, confs: list[Confirmation]) -> str:
    for c in reversed(confs):
        if c.fix:
            return c.fix
    return (row.summary or "").split(".")[0][:160]


def _advisory_url(vuln_id: str) -> str:
    return f"https://osv.dev/vulnerability/{vuln_id}"


# ─── jinja2 setup ───────────────────────────────────────────────────


def _template_env() -> Environment:
    template_dir = Path(__file__).resolve().parent / "templates"
    return Environment(
        loader=FileSystemLoader(template_dir),
        autoescape=select_autoescape(["html", "j2"]),
        trim_blocks=True,
        lstrip_blocks=True,
        undefined=StrictUndefined,
    )


def _helpers_namespace() -> SimpleNamespace:
    """Build the `helpers` object passed to both templates."""
    return SimpleNamespace(
        severity_class=lambda sev: SEV_CLASS.get(sev, "sev-unk"),
        effective_verdict=_effective_verdict,
        fix_for=_fix_for,
        scanner_short=lambda s: SCANNER_SHORT.get(s, s),
        advisory_url=_advisory_url,
        row_key=lambda r: r.key(),
        project_sev_counts=_project_sev_counts,
        project_actionable=_project_actionable,
    )


# ─── public entry ───────────────────────────────────────────────────


def _skill_root() -> Path:
    """Locate the skill root from this lib file's path."""
    return Path(__file__).resolve().parent


def _harness_root_from_scan_dir(scan_dir: Path) -> Path:
    """Walk up from scan_dir to find project-defs/registry.toml."""
    p = scan_dir.resolve()
    while p != p.parent:
        if (p / "project-defs" / "registry.toml").exists():
            return p
        p = p.parent
    return scan_dir.resolve()


def _ioc_matches(
    scan_dir: Path,
    unified: list[UnifiedRow],
    tools_manifest: ToolsManifest,
) -> tuple[list[IocCampaign], list[IocMatch]]:
    """Load every campaign from `<skill>/data/iocs/*.json` and cross-check.

    Two match paths:
      1. Against the deduped unified findings (scanner already flagged it)
      2. Against each scanned project's raw dep files (scanner missed it
         because the IOC isn't in OSV/etc. yet, but the package@version
         is sitting in the lockfile)

    The dedup step at the end keeps one row per (campaign, project,
    package, version, ecosystem) tuple, preferring the scanner-detected
    entry when both paths fire.
    """
    campaigns = load_campaigns(_skill_root())
    if not campaigns:
        return [], []
    from_scan = match_against_unified(campaigns, unified)
    harness_root = _harness_root_from_scan_dir(scan_dir)
    from_deps = match_against_project_deps(
        campaigns,
        harness_root,
        tools_manifest.projects_scanned,
    )
    seen: dict[tuple[str, str, str, str, str], IocMatch] = {}
    for m in from_scan:
        seen[m.key()] = m
    for m in from_deps:
        seen.setdefault(m.key(), m)
    return campaigns, sorted(
        seen.values(),
        key=lambda m: (m.campaign, m.project, m.package, m.version),
    )


def _campaign_slug(campaign_name: str) -> str:
    """Derive a filename-safe slug from a campaign string."""
    slug = re.sub(r"[^a-zA-Z0-9]+", "-", campaign_name).strip("-").lower()
    return slug or "campaign"


def _render_campaigns(
    scan_dir: Path,
    campaigns: list[IocCampaign],
    ioc_matches: list[IocMatch],
    env: Environment,
) -> list[tuple[IocCampaign, Path, Path]]:
    """Render one standalone <md, html> pair per loaded campaign into iocs/.

    Returns [(campaign, md_path, html_path)] so the main report can link
    to each campaign's full IOC listing.
    """
    iocs_dir = scan_dir / "iocs"
    iocs_dir.mkdir(parents=True, exist_ok=True)
    md_tpl = env.get_template("campaign.md.j2")
    html_tpl = env.get_template("campaign.html.j2")
    matches_by_campaign: dict[str, list[IocMatch]] = defaultdict(list)
    for m in ioc_matches:
        matches_by_campaign[m.campaign].append(m)
    out: list[tuple[IocCampaign, Path, Path]] = []
    for camp in campaigns:
        slug = _campaign_slug(camp.campaign)
        md_path = iocs_dir / f"{slug}.md"
        html_path = iocs_dir / f"{slug}.html"
        ctx = {"campaign": camp, "matches": matches_by_campaign.get(camp.campaign, [])}
        md_path.write_text(md_tpl.render(**ctx))
        html_path.write_text(html_tpl.render(**ctx))
        out.append((camp, md_path, html_path))
    return out


def _conflict_rows(
    unified: list[UnifiedRow],
    confs_by_key: dict[RowKey, list[Confirmation]],
) -> list[tuple[UnifiedRow, dict[str, str]]]:
    """List (row, {agent_id: verdict}) for findings where agents disagree."""
    out: list[tuple[UnifiedRow, dict[str, str]]] = []
    for row in unified:
        confs = confs_by_key.get(row.key(), [])
        if _has_conflict(confs):
            out.append((row, _verdicts_per_agent(confs)))
    return out


def render(scan_dir: Path) -> RenderResult:
    """Render REPORT.{md,html} + XRAY.{md,html} + iocs/<slug>.{md,html}.

    Two separate top-level reports:
      - REPORT.* — traditional CVE/GHSA findings (one row per UnifiedRow)
      - XRAY.*   — supply-chain attack matches against loaded IOC feeds

    Per-campaign IOC pages under `iocs/` are linked from XRAY.* and
    always written so reviewers can browse the full malicious-package
    list for each campaign standalone.
    """
    unified_path = scan_dir / "merged" / "unified.jsonl"
    if not unified_path.exists():
        msg = f"unified.jsonl not found: {unified_path} (run merge first)"
        raise FileNotFoundError(msg)

    unified = load_jsonl(unified_path, UnifiedRow)
    confs = load_jsonl(scan_dir / "confirmations.jsonl", Confirmation)
    confs_by_key = _group_confirmations(confs)
    tools_manifest = load_tools_manifest(scan_dir / "tools.json")
    campaigns, ioc_matches = _ioc_matches(scan_dir, unified, tools_manifest)
    ioc_matches_by_project: dict[str, list[IocMatch]] = defaultdict(list)
    for m in ioc_matches:
        ioc_matches_by_project[m.project].append(m)

    env = _template_env()
    campaign_pages = _render_campaigns(scan_dir, campaigns, ioc_matches, env)
    campaign_links = [
        {
            "campaign": camp,
            "md_rel": str(md.relative_to(scan_dir)),
            "html_rel": str(html.relative_to(scan_dir)),
        }
        for camp, md, html in campaign_pages
    ]
    conflicts = _conflict_rows(unified, confs_by_key)

    common_ctx: dict[str, object] = {
        "today": dt.datetime.now(tz=dt.UTC).strftime("%Y-%m-%d"),
        "summary": _summary(unified, confs_by_key),
        "tools_manifest": tools_manifest,
        "scanner_total": len(tools_manifest.tools) or len(SCANNERS),
        "scanner_total_for": tools_manifest.scanner_total_for,
        "confs_by_key": confs_by_key,
        "audit_trail": sorted(confs, key=lambda c: c.confirmed_at),
        "helpers": _helpers_namespace(),
        "conflicts": conflicts,
        "conflict_count": len(conflicts),
    }

    # ── main REPORT (CVE/GHSA only — no supply-chain section) ──
    main_ctx = dict(common_ctx)
    main_ctx["by_project"] = _by_project(unified, confs_by_key)
    md_path = scan_dir / "REPORT.md"
    html_path = scan_dir / "report.html"
    md_path.write_text(env.get_template("report.md.j2").render(**main_ctx))
    html_path.write_text(env.get_template("report.html.j2").render(**main_ctx))

    # ── XRAY report (supply-chain matches across all loaded campaigns) ──
    xray_ctx = dict(common_ctx)
    xray_ctx["ioc_campaigns"] = campaigns
    xray_ctx["ioc_matches"] = ioc_matches
    xray_ctx["ioc_matches_by_project"] = dict(ioc_matches_by_project)
    xray_ctx["campaign_links"] = campaign_links
    xray_md_path = scan_dir / "XRAY.md"
    xray_html_path = scan_dir / "xray.html"
    xray_md_path.write_text(env.get_template("xray.md.j2").render(**xray_ctx))
    xray_html_path.write_text(env.get_template("xray.html.j2").render(**xray_ctx))

    return RenderResult(
        md_path=md_path,
        html_path=html_path,
        xray_md_path=xray_md_path,
        xray_html_path=xray_html_path,
        unified_count=len(unified),
        confirmations_count=len(confs),
        conflicts_count=len(conflicts),
        ioc_match_count=len(ioc_matches),
    )


# Re-exports for backwards-compat (and to quiet unused-import lints):
_ = ToolsManifest
_ = SCANNERS
