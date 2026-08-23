"""Merge normalized findings by alias chain, emit deduped views.

Pure library module. `merge.merge(scan_dir)` reads
`<scan_dir>/normalized/findings.jsonl` (one `Finding` per line) and
writes:
  - `<scan_dir>/merged/unified.jsonl`       (one `UnifiedRow` per cluster)
  - `<scan_dir>/merged/coverage_matrix.tsv` (scanner x finding grid)
  - `<scan_dir>/merged/per_project.tsv`     (same data, project-sorted)

Returns `MergeStats` (raw + unified counts + severity + coverage).

Dedup strategy:
  1. Union-Find over every ID that ever appears together in any single
     finding's `all_ids` set → alias clusters collapse (e.g. {GHSA-x,
     CVE-y, PYSEC-z} share one canonical id).
  2. Group findings by `(project, package, version, cluster_canonical_id)`.
  3. Per group: union the scanners, pick highest severity, longest summary.
"""

from __future__ import annotations

import re
from collections import defaultdict
from typing import TYPE_CHECKING

from .models import (
    SCANNERS,
    SEV_RANK,
    Finding,
    MergeStats,
    ScannerName,
    Severity,
    UnifiedRow,
    load_jsonl,
    write_jsonl,
)

if TYPE_CHECKING:
    from pathlib import Path

ID_PREF: tuple[str, ...] = ("CVE-", "GHSA-", "PYSEC-", "MAL-", "OSV-", "BIT-")


def _canonical_id(ids: list[str]) -> str:
    clean = sorted({i for i in ids if i})
    for prefix in ID_PREF:
        for i in clean:
            if i.startswith(prefix):
                return i
    return clean[0] if clean else "UNKNOWN"


def _normalize_source_file(s: str) -> str:
    """Strip absolute path prefix so cross-scanner files compare equal."""
    if not s:
        return ""
    m = re.search(r"projects/[^/]+/[^/]+/(.+)$", s)
    return m.group(1) if m else s


# ─── Union-Find over alias IDs ──────────────────────────────────────


class _UnionFind:
    def __init__(self) -> None:
        self.parent: dict[str, str] = {}

    def find(self, x: str) -> str:
        while self.parent.get(x, x) != x:
            self.parent[x] = self.parent.get(self.parent[x], self.parent[x])
            x = self.parent[x]
        return x

    def union(self, *ids: str) -> None:
        clean = [i for i in ids if i]
        if not clean:
            return
        for i in clean:
            self.parent.setdefault(i, i)
        roots = {self.find(i) for i in clean}
        if len(roots) <= 1:
            return
        canon = _canonical_id(list(roots))
        for r in roots:
            self.parent[r] = canon

    def cluster_for(self, ids: list[str]) -> str:
        seeds = [i for i in ids if i]
        if not seeds:
            return "UNKNOWN"
        roots = {self.find(i) for i in seeds if i in self.parent}
        return _canonical_id(list(roots)) if roots else _canonical_id(seeds)


def _build_clusters(findings: list[Finding]) -> _UnionFind:
    uf = _UnionFind()
    for r in findings:
        ids = list(r.all_ids)
        if r.primary_id:
            ids.append(r.primary_id)
        for i in ids:
            uf.parent.setdefault(i, i)
        if len(ids) > 1:
            uf.union(*ids)
    return uf


def _bucket_findings(
    findings: list[Finding],
    uf: _UnionFind,
) -> dict[tuple[str, str, str, str], list[Finding]]:
    buckets: dict[tuple[str, str, str, str], list[Finding]] = defaultdict(list)
    for r in findings:
        ids = list(r.all_ids)
        if r.primary_id:
            ids.append(r.primary_id)
        cluster = uf.cluster_for(ids)
        key = (r.project, r.package, r.version, cluster)
        buckets[key].append(r)
    return buckets


def _highest_sev(severities: set[str]) -> Severity:
    for s in ("CRITICAL", "HIGH", "MODERATE", "LOW"):
        if s in severities:
            match s:
                case "CRITICAL":
                    return "CRITICAL"
                case "HIGH":
                    return "HIGH"
                case "MODERATE":
                    return "MODERATE"
                case "LOW":
                    return "LOW"
    return "UNKNOWN"


def _unify(buckets: dict[tuple[str, str, str, str], list[Finding]]) -> list[UnifiedRow]:
    out: list[UnifiedRow] = []
    for key, rows in buckets.items():
        proj, pkg, ver, cluster = key
        scanners_hit: list[ScannerName] = sorted({r.scanner for r in rows})
        best_sev = _highest_sev({r.severity for r in rows})
        all_ids: set[str] = set()
        for r in rows:
            all_ids.update(r.all_ids)
            if r.primary_id:
                all_ids.add(r.primary_id)
        # Longest non-empty summary.
        summary = ""
        for r in rows:
            if r.summary and len(r.summary) > len(summary):
                summary = r.summary
        src_files = sorted({_normalize_source_file(r.source_file) for r in rows if r.source_file})
        eco = next((r.ecosystem for r in rows if r.ecosystem and r.ecosystem != "?"), "?")
        out.append(
            UnifiedRow(
                project=proj,
                package=pkg,
                version=ver,
                ecosystem=eco,
                cluster_id=cluster,
                all_ids=sorted(all_ids),
                severity=best_sev,
                scanners=scanners_hit,
                scanner_count=len(scanners_hit),
                source_files=src_files,
                summary=summary[:240],
            )
        )
    out.sort(key=lambda r: (SEV_RANK[r.severity], -r.scanner_count, r.project, r.package))
    return out


# ─── tabular outputs ────────────────────────────────────────────────


def _write_coverage_matrix(unified: list[UnifiedRow], out: Path) -> None:
    with out.open("w") as fp:
        fp.write(
            "severity\tscanners_hit\tproject\tpackage\tversion\tcluster_id\t"
            + "\t".join(SCANNERS)
            + "\tall_ids\tsummary\n"
        )
        for r in unified:
            cells: list[str] = [
                r.severity,
                str(r.scanner_count),
                r.project,
                r.package,
                r.version,
                r.cluster_id,
            ]
            cells.extend("✓" if s in r.scanners else "" for s in SCANNERS)
            cells.append(",".join(r.all_ids))
            cells.append(r.summary)
            fp.write("\t".join(cells) + "\n")


def _write_per_project(unified: list[UnifiedRow], out: Path) -> None:
    sorted_rows = sorted(
        unified,
        key=lambda r: (r.project, SEV_RANK[r.severity], r.package),
    )
    with out.open("w") as fp:
        fp.write("project\tseverity\tpackage\tversion\tcluster_id\tscanners\tall_ids\n")
        for r in sorted_rows:
            fp.write(
                f"{r.project}\t{r.severity}\t{r.package}\t{r.version}\t{r.cluster_id}\t"
                f"{','.join(r.scanners)}\t{','.join(r.all_ids)}\n"
            )


def merge(scan_dir: Path) -> MergeStats:
    """Merge normalized findings by alias chain. Writes unified + matrices."""
    norm = scan_dir / "normalized" / "findings.jsonl"
    if not norm.exists():
        msg = f"normalized findings not found: {norm}"
        raise FileNotFoundError(msg)
    merged_dir = scan_dir / "merged"
    merged_dir.mkdir(parents=True, exist_ok=True)

    findings = load_jsonl(norm, Finding)
    uf = _build_clusters(findings)
    buckets = _bucket_findings(findings, uf)
    unified = _unify(buckets)

    unified_path = merged_dir / "unified.jsonl"
    coverage_path = merged_dir / "coverage_matrix.tsv"
    per_project_path = merged_dir / "per_project.tsv"
    write_jsonl(unified_path, unified)
    _write_coverage_matrix(unified, coverage_path)
    _write_per_project(unified, per_project_path)

    sev_counts: dict[str, int] = defaultdict(int)
    for r in unified:
        sev_counts[r.severity] += 1
    cov_counts: dict[int, int] = defaultdict(int)
    for r in unified:
        cov_counts[r.scanner_count] += 1
    return MergeStats(
        raw_count=len(findings),
        unified_count=len(unified),
        severity_counts=dict(sev_counts),
        coverage_counts=dict(cov_counts),
        unified_path=unified_path,
        coverage_path=coverage_path,
        per_project_path=per_project_path,
    )
