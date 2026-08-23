"""Load supply-chain attack IOC campaigns and cross-check against scan output.

Why a separate module: traditional CVE/GHSA findings (in legitimate code)
and supply-chain attacks (intentionally-malicious package releases) are
fundamentally different remediation classes. The CVE feeds catch the
former; commercial vendors and ad-hoc IOC lists catch the latter
**hours-to-days before** OSV-Malicious feeds ingest them.

This module reads `data/iocs/*.json` campaign files and matches their
`(package, version)` tuples against scanned projects with two strictness
levels:

  1. `exact`     — `(name, version)` both match a campaign entry → the
                   project is running a confirmed malicious release.
  2. `name-only` — name matches but version doesn't → safe today, but the
                   project depends on a lib actively under attack; pin
                   tight and audit upgrades.

Two input sources feed the matcher:
- The deduped UnifiedRow set (what the scanners already flagged)
- The project's raw dep files (lockfiles + manifests) — catches the case
  where the scanner DB has no entry yet but the malicious version is
  sitting in the lockfile

A match here is intentionally NOT severity-graded against CVSS — supply
chain attacks bypass the standard severity model. Treat every exact
match as CRITICAL unless an agent confirms otherwise in Phase 2.
"""

from __future__ import annotations

import json
import os
import re
import tomllib
from collections import defaultdict
from pathlib import Path

from .models import (
    IocCampaign,
    IocMatch,
    UnifiedRow,
)

# ─── loaders ────────────────────────────────────────────────────────


def load_campaigns(skill_root: Path) -> list[IocCampaign]:
    """Read every `data/iocs/*.json` from the skill dir; sorted by file name."""
    ioc_dir = skill_root / "data" / "iocs"
    if not ioc_dir.exists():
        return []
    out: list[IocCampaign] = []
    for f in sorted(ioc_dir.glob("*.json")):
        try:
            out.append(IocCampaign.model_validate_json(f.read_text()))
        except json.JSONDecodeError, ValueError:
            continue
    return out


# ─── ecosystem normalization ────────────────────────────────────────


def _ecosystem_eq(scanner_eco: str, ioc_eco: str) -> bool:
    """Case-insensitive ecosystem name match across scanner output dialects."""
    s = scanner_eco.lower()
    i = ioc_eco.lower()
    if s == i:
        return True
    aliases = {
        "pypi": {"pip", "python", "pypi"},
        "npm": {"npm", "node", "javascript"},
    }
    for canon, group in aliases.items():
        if i == canon and s in group:
            return True
        if s == canon and i in group:
            return True
    return False


# ─── matching path A: against scanner-emitted UnifiedRow ────────────


def match_against_unified(
    campaigns: list[IocCampaign],
    unified: list[UnifiedRow],
) -> list[IocMatch]:
    """One IocMatch per (campaign x unified row) where pkg+ver+ecosystem agrees.

    All matches here are `match_kind == "exact"` by construction (the
    scanner only emits findings for installed versions).
    """
    out: list[IocMatch] = []
    for camp in campaigns:
        for eco_name, pkgs in camp.ecosystems.items():
            for ioc_pkg in pkgs:
                for ver in ioc_pkg.versions:
                    out.extend(
                        IocMatch(
                            campaign=camp.campaign,
                            source=camp.source,
                            project=row.project,
                            package=row.package,
                            version=row.version,
                            ecosystem=eco_name,
                            xray_id=ioc_pkg.xray_id,
                            where=", ".join(row.source_files) or "(unified)",
                            detected_by_scanner=True,
                            scanner_clusters=[row.cluster_id],
                            match_kind="exact",
                            flagged_versions=list(ioc_pkg.versions),
                        )
                        for row in unified
                        if _ecosystem_eq(row.ecosystem, eco_name) and row.package == ioc_pkg.name and row.version == ver
                    )
    return out


# ─── matching path B: against raw dep files (lockfiles + manifests) ──


_NPM_LOCKS: tuple[str, ...] = ("package-lock.json", "yarn.lock", "pnpm-lock.yaml", "package.json")
_PYPI_LOCKS: tuple[str, ...] = ("uv.lock", "poetry.lock", "Pipfile.lock")
_PYPI_REQ_RE: re.Pattern[str] = re.compile(r"requirements([_-].*)?\.txt$")
_SKIP_PREFIXES: tuple[str, ...] = (".venv", "node_modules", ".cache", ".git", "__pycache__")


def match_against_project_deps(
    campaigns: list[IocCampaign],
    harness_root: Path,
    projects: list[str],
) -> list[IocMatch]:
    """Match campaign packages against each project's installed dep files.

    Walk each project's worktree once per ecosystem, extract every
    installed (name, version) pair, then for each IOC name in any
    campaign emit either:

      - one `exact` match per (project, package, version, where) triple
        where version is in the campaign's flagged versions, OR
      - one `name-only` match per (project, package, version, where)
        triple where the name matches some campaign entry but version
        doesn't.

    Cost is O(projects x ecosystems) walks regardless of campaign count.
    A project's multi-GB .venv is pruned at directory level (see _walk_deps).
    """
    out: list[IocMatch] = []
    for proj in projects:
        worktree = _resolve_worktree(harness_root, proj)
        if worktree is None or not worktree.exists():
            continue
        # Per-ecosystem cache: installed[(eco, name)] = {(version, where), …}
        installed = _installed_packages(worktree)
        for camp in campaigns:
            for eco_name, pkgs in camp.ecosystems.items():
                eco_lc = eco_name.lower()
                for ioc_pkg in pkgs:
                    flagged = set(ioc_pkg.versions)
                    found = installed.get((eco_lc, ioc_pkg.name), set())
                    out.extend(
                        IocMatch(
                            campaign=camp.campaign,
                            source=camp.source,
                            project=proj,
                            package=ioc_pkg.name,
                            version=ver,
                            ecosystem=eco_name,
                            xray_id=ioc_pkg.xray_id,
                            where=where,
                            detected_by_scanner=False,
                            match_kind="exact" if ver in flagged else "name-only",
                            flagged_versions=sorted(flagged),
                        )
                        for ver, where in sorted(found)
                    )
    return out


def _resolve_worktree(harness_root: Path, project: str) -> Path | None:
    """Find the project's registered stable checkout."""
    project_file = harness_root / "project-defs" / project / "project.toml"
    try:
        config = tomllib.loads(project_file.read_text())
    except OSError, tomllib.TOMLDecodeError:
        return None
    base = harness_root / "projects" / project
    if config.get("kind") != "external":
        return base
    return base


def _installed_packages(worktree: Path) -> dict[tuple[str, str], set[tuple[str, str]]]:
    """Return {(ecosystem, name): {(version, where_path), …}} for this worktree.

    Uses `os.walk` with directory pruning so a project's multi-GB `.venv`
    doesn't get traversed.
    """
    result: dict[tuple[str, str], set[tuple[str, str]]] = defaultdict(set)
    for fpath in _walk_deps(worktree):
        rel = str(fpath.relative_to(worktree))
        name = fpath.name
        try:
            text = fpath.read_text(errors="ignore")
        except OSError:
            continue
        if name in _NPM_LOCKS:
            for pkg_name, version in _extract_npm(text, name):
                result[("npm", pkg_name)].add((version, rel))
        elif name in _PYPI_LOCKS or _PYPI_REQ_RE.search(name):
            for pkg_name, version in _extract_pypi(text, name):
                result[("pypi", pkg_name)].add((version, rel))
    return result


def _walk_deps(worktree: Path) -> list[Path]:
    """Yield candidate dep files, pruning skip-prefix dirs at the dirname level."""
    out: list[Path] = []
    for dirpath, dirnames, filenames in os.walk(worktree):
        dirnames[:] = [d for d in dirnames if not any(d.startswith(pre) for pre in _SKIP_PREFIXES)]
        out.extend(
            Path(dirpath) / fname
            for fname in filenames
            if fname in _NPM_LOCKS or fname in _PYPI_LOCKS or _PYPI_REQ_RE.search(fname)
        )
    return out


# ─── ecosystem-specific extractors ──────────────────────────────────


_NPM_PKG_LOCK_RE = re.compile(
    r'"node_modules/(?P<name>(?:@[^/"]+/)?[^"]+)"\s*:\s*\{[^}]*?"version"\s*:\s*"(?P<ver>[^"]+)"',
    re.DOTALL,
)
_NPM_PKG_JSON_DEP_RE = re.compile(
    r'"(?P<name>(?:@[^/"]+/)?[^"]+)"\s*:\s*"(?:\^|~|=|>|<)?(?P<ver>[\dvx][^"]*)"',
)


def _extract_npm(text: str, filename: str) -> list[tuple[str, str]]:
    """Best-effort (name, version) extraction for npm lockfiles + package.json."""
    out: list[tuple[str, str]] = []
    if filename == "package-lock.json":
        out.extend((m.group("name"), m.group("ver")) for m in _NPM_PKG_LOCK_RE.finditer(text))
    elif filename == "package.json":
        # Best-effort: scan only the standard dependency sections.
        sections = ("dependencies", "devDependencies", "peerDependencies", "optionalDependencies")
        for section in sections:
            sec_match = re.search(rf'"{section}"\s*:\s*\{{([^}}]*)\}}', text, re.DOTALL)
            if not sec_match:
                continue
            out.extend((m.group("name"), m.group("ver")) for m in _NPM_PKG_JSON_DEP_RE.finditer(sec_match.group(1)))
    elif filename == "yarn.lock":
        # yarn.lock entries look like:  "@scope/name@^1.2.3":\n  version "1.2.4"
        yarn_entry_re = re.compile(
            r'^"?(?P<spec>(?:@[^/]+/)?[^@\s"]+)@[^"\n]+"?:\s*\n'
            r'(?:\s{2}.*\n)*?\s{2}version\s+"(?P<ver>[^"]+)"',
            re.MULTILINE,
        )
        out.extend((entry.group("spec"), entry.group("ver")) for entry in yarn_entry_re.finditer(text))
    return out


_PYPI_TOML_PKG_RE = re.compile(
    r'^\s*name\s*=\s*"(?P<name>[^"]+)"\s*\n\s*version\s*=\s*"(?P<ver>[^"]+)"',
    re.MULTILINE,
)
_PYPI_REQ_LINE_RE = re.compile(
    r"^\s*(?P<name>[A-Za-z0-9][A-Za-z0-9._-]*)\s*==\s*(?P<ver>[^\s;#]+)",
    re.MULTILINE,
)


def _extract_pypi(text: str, filename: str) -> list[tuple[str, str]]:
    """Best-effort (name, version) extraction for Python lockfiles + requirements."""
    if filename in _PYPI_LOCKS:
        return [(m.group("name").lower(), m.group("ver")) for m in _PYPI_TOML_PKG_RE.finditer(text)]
    return [(m.group("name").lower(), m.group("ver")) for m in _PYPI_REQ_LINE_RE.finditer(text)]
