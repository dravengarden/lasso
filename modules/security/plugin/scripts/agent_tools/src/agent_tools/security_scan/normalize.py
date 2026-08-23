"""Normalize each scanner's JSON output to a common pydantic schema.

Pure library module. The orchestrator (`scan.py`) imports `normalize`
and feeds it the scan directory.

Behavior:
- Reads `<scan_dir>/raw/{osv-scanner,pip-audit,trivy,grype}/*.json`
- Per-scanner parser maps native JSON → `Finding`
- Writes `<scan_dir>/normalized/findings.jsonl` (one `Finding` per line)
- Returns `NormalizeStats` (per-scanner counts + total + output path)

Per-scanner blind spots and quirks are documented inline in each parser.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import TYPE_CHECKING, Any, cast

from .models import (
    Finding,
    NormalizeStats,
    Severity,
    append_jsonl,
)

if TYPE_CHECKING:
    from collections.abc import Callable

ID_PREF: tuple[str, ...] = ("CVE-", "GHSA-", "PYSEC-", "MAL-", "OSV-", "BIT-")
VALID_SEV: frozenset[str] = frozenset({"CRITICAL", "HIGH", "MODERATE", "LOW", "UNKNOWN"})


def _canonical_id(ids: list[str]) -> str:
    clean = [i for i in ids if i]
    for prefix in ID_PREF:
        for i in clean:
            if i.startswith(prefix):
                return i
    return clean[0] if clean else "UNKNOWN"


def _norm_sev(s: str | None) -> Severity:
    if not s:
        return "UNKNOWN"
    up = s.upper()
    match up:
        case "CRITICAL":
            return "CRITICAL"
        case "HIGH":
            return "HIGH"
        case "MODERATE" | "MEDIUM":
            return "MODERATE"
        case "LOW":
            return "LOW"
        case _:
            return "UNKNOWN"


def _project_from_filename(f: Path, scanner: str) -> str:
    """Resolve scanner output filename → project name.

    osv-scanner / trivy / grype write `raw/<scanner>/<project>.json`.
    pip-audit uses `<tag>.json` where tag is `<project>__<reqfile>`.
    We take everything before `__` (or first `_-`) as the project name.
    """
    stem = f.stem
    if scanner in {"osv-scanner", "trivy", "grype"}:
        return stem
    if "__" in stem:
        return stem.split("__", 1)[0]
    return re.split(r"[_-]", stem, maxsplit=1)[0]


def _ids_from(primary: str | None, aliases: list[str] | None) -> list[str]:
    out: list[str] = []
    if primary:
        out.append(primary)
    if aliases:
        out.extend(aliases)
    return out


def _emit(rec: Finding, out_path: Path) -> None:
    append_jsonl(out_path, rec)


# ─── per-scanner parsers ────────────────────────────────────────────


def _parse_osv(raw: Path, out_path: Path) -> None:
    for f in sorted((raw / "osv-scanner").glob("*.json")):
        try:
            data: dict[str, Any] = json.loads(f.read_text())
        except json.JSONDecodeError, ValueError:
            continue
        proj = _project_from_filename(f, "osv-scanner")
        results = cast("list[dict[str, Any]]", data.get("results") or [])
        for res in results:
            source = cast("dict[str, Any]", res.get("source") or {})
            src_path = cast("str", source.get("path") or "?")
            for pkg in cast("list[dict[str, Any]]", res.get("packages") or []):
                p = cast("dict[str, Any]", pkg.get("package") or {})
                for v in cast("list[dict[str, Any]]", pkg.get("vulnerabilities") or []):
                    vuln_id = cast("str | None", v.get("id"))
                    aliases = cast("list[str] | None", v.get("aliases"))
                    ids = _ids_from(vuln_id, aliases)
                    db_specific = cast("dict[str, Any]", v.get("database_specific") or {})
                    sev_list = cast("list[dict[str, Any]]", v.get("severity") or [])
                    first_score = cast("str | None", sev_list[0].get("score")) if sev_list else None
                    sev = cast("str | None", db_specific.get("severity") or first_score)
                    _emit(
                        Finding(
                            scanner="osv-scanner",
                            project=proj,
                            source_file=src_path,
                            package=cast("str", p.get("name") or "?"),
                            ecosystem=cast("str", p.get("ecosystem") or "?"),
                            version=cast("str", p.get("version") or "?"),
                            primary_id=_canonical_id(ids),
                            all_ids=sorted(set(filter(None, ids))),
                            severity=_norm_sev(sev),
                            summary=(cast("str", v.get("summary") or ""))[:200],
                        ),
                        out_path,
                    )


def _parse_pip_audit(raw: Path, out_path: Path) -> None:
    for f in sorted((raw / "pip-audit").glob("*.json")):
        try:
            data: dict[str, Any] = json.loads(f.read_text())
        except json.JSONDecodeError, ValueError:
            continue
        tag = f.stem
        proj = _project_from_filename(f, "pip-audit")
        for dep in cast("list[dict[str, Any]]", data.get("dependencies") or []):
            name = cast("str", dep.get("name") or "?")
            ver = cast("str", dep.get("version") or "?")
            for v in cast("list[dict[str, Any]]", dep.get("vulns") or []):
                vuln_id = cast("str | None", v.get("id"))
                aliases = cast("list[str] | None", v.get("aliases"))
                ids = _ids_from(vuln_id, aliases)
                _emit(
                    Finding(
                        scanner="pip-audit",
                        project=proj,
                        source_file=tag,
                        package=name,
                        ecosystem="PyPI",
                        version=ver,
                        primary_id=_canonical_id(ids),
                        all_ids=sorted(set(filter(None, ids))),
                        # pip-audit doesn't surface severity in its JSON.
                        severity="UNKNOWN",
                        summary=(cast("str", v.get("description") or ""))[:200],
                    ),
                    out_path,
                )


def _parse_trivy(raw: Path, out_path: Path) -> None:
    for f in sorted((raw / "trivy").glob("*.json")):
        try:
            data: dict[str, Any] = json.loads(f.read_text())
        except json.JSONDecodeError, ValueError:
            continue
        proj = _project_from_filename(f, "trivy")
        for res in cast("list[dict[str, Any]]", data.get("Results") or []):
            target = cast("str", res.get("Target") or "?")
            for v in cast("list[dict[str, Any]]", res.get("Vulnerabilities") or []):
                primary = cast("str | None", v.get("VulnerabilityID")) or "UNKNOWN"
                ids = [primary]
                purl = cast(
                    "str | None",
                    cast("dict[str, Any]", v.get("PkgIdentifier") or {}).get("PURL"),
                )
                eco = "?"
                if purl and purl.startswith("pkg:"):
                    eco = purl[4:].split("/", 1)[0]
                _emit(
                    Finding(
                        scanner="trivy",
                        project=proj,
                        source_file=target,
                        package=cast("str", v.get("PkgName") or "?"),
                        ecosystem=eco,
                        version=cast("str", v.get("InstalledVersion") or "?"),
                        primary_id=primary,
                        all_ids=sorted(set(filter(None, ids))),
                        severity=_norm_sev(cast("str | None", v.get("Severity"))),
                        summary=(cast("str", v.get("Title") or v.get("Description") or ""))[:200],
                    ),
                    out_path,
                )


def _parse_grype(raw: Path, out_path: Path) -> None:
    for f in sorted((raw / "grype").glob("*.json")):
        try:
            data: dict[str, Any] = json.loads(f.read_text())
        except json.JSONDecodeError, ValueError:
            continue
        proj = _project_from_filename(f, "grype")
        for m in cast("list[dict[str, Any]]", data.get("matches") or []):
            v = cast("dict[str, Any]", m.get("vulnerability") or {})
            art = cast("dict[str, Any]", m.get("artifact") or {})
            related = cast("list[dict[str, Any]]", m.get("relatedVulnerabilities") or [])
            primary = cast("str | None", v.get("id"))
            ids: list[str] = ([primary] if primary else []) + [cast("str", r.get("id")) for r in related if r.get("id")]
            locs = cast("list[dict[str, Any]]", art.get("locations") or [])
            src = cast("str", locs[0].get("path") or "?") if locs else "?"
            _emit(
                Finding(
                    scanner="grype",
                    project=proj,
                    source_file=src,
                    package=cast("str", art.get("name") or "?"),
                    ecosystem=cast("str", art.get("type") or "?"),
                    version=cast("str", art.get("version") or "?"),
                    primary_id=_canonical_id(ids),
                    all_ids=sorted(set(filter(None, ids))),
                    severity=_norm_sev(cast("str | None", v.get("severity"))),
                    summary=(cast("str", v.get("description") or ""))[:200],
                ),
                out_path,
            )


type ParserFn = Callable[[Path, Path], None]

PARSERS: list[tuple[str, ParserFn]] = [
    ("osv-scanner", _parse_osv),
    ("pip-audit", _parse_pip_audit),
    ("trivy", _parse_trivy),
    ("grype", _parse_grype),
]


def normalize(scan_dir: Path) -> NormalizeStats:
    """Normalize every scanner's JSON output into a single JSONL of `Finding`."""
    raw = scan_dir / "raw"
    out = scan_dir / "normalized" / "findings.jsonl"
    out.parent.mkdir(parents=True, exist_ok=True)
    if not raw.exists():
        msg = f"raw dir not found: {raw}"
        raise FileNotFoundError(msg)
    # Truncate so re-runs don't double-count.
    out.write_text("")

    per_scanner: dict[str, int] = {}
    total = 0
    for scanner_name, parser in PARSERS:
        if not any((raw / scanner_name).glob("*.json")):
            continue
        parser(raw, out)
        after = sum(1 for _ in out.open())
        per_scanner[scanner_name] = after - total
        total = after
    return NormalizeStats(per_scanner=per_scanner, total=total, output_path=out)


_ = sys
