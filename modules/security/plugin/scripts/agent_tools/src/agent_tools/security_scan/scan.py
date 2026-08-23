"""Phase 1 of the `security-scan` skill — multi-scanner audit + report.

Subcommands:
  discover                       list candidate projects (those with scannable deps)
  run --projects A,B,C            run the 4-scanner pipeline + normalize + merge + render
  run --all-projects              same, against every scannable project

Output layout (one dir per scan, gitignored under harness `.scratch/`):

    .scratch/security-scan-YYYY-MM-DD/
    ├── raw/<scanner>/<project>.{json,stderr}
    ├── normalized/findings.jsonl
    ├── merged/{unified.jsonl, coverage_matrix.tsv, per_project.tsv}
    ├── tools.json                 ← scanner versions + vuln-db descriptions
    ├── confirmations.jsonl        ← empty initially; Phase 2 appends
    ├── REPORT.md                  ← Jinja2-rendered markdown report
    └── report.html                ← self-contained HTML report

Assumes the 4 scanners + `uv` are on PATH inside the dev shell:
  osv-scanner / pip-audit / trivy / grype / uv
"""

from __future__ import annotations

import datetime as dt
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Annotated

import typer
from rich.console import Console
from rich.panel import Panel
from rich.progress import (
    Progress,
    SpinnerColumn,
    TaskID,
    TextColumn,
    TimeElapsedColumn,
)
from rich.table import Table

from .discover import (
    discover,
    harness_root_default,
)
from .merge import (
    merge,
)
from .models import (
    SCANNERS,
    DepTarget,
    Finding,
    ProjectEntry,
    ScannerName,
    ScannerVersions,
    ToolEntry,
    ToolsManifest,
    UnifiedRow,
    load_jsonl,
)
from .normalize import (
    normalize,
)

console = Console()
REQUIRED_TOOLS: tuple[str, ...] = ("osv-scanner", "pip-audit", "trivy", "grype", "uv")
app = typer.Typer(
    no_args_is_help=True,
    add_completion=False,
    pretty_exceptions_show_locals=False,
    rich_markup_mode="markdown",
    help="Multi-scanner dependency vulnerability audit for the harness projects.",
)


# ─── helpers ─────────────────────────────────────────────────────────


def _parse_projects(value: str) -> list[str]:
    if not value:
        return []
    return [p.strip() for p in value.split(",") if p.strip()]


def _require_tools() -> None:
    """Fail before scanning when the pinned shell is incomplete."""
    missing = [tool for tool in REQUIRED_TOOLS if shutil.which(tool) is None]
    if missing:
        console.print(f"[red]missing scanner tools:[/red] {', '.join(missing)}")
        console.print("Use the bundled security-scan wrapper or enter the plugin dev shell with `nix develop`.")
        raise typer.Exit(code=1)


def _scan_dir_default(out: str | None) -> Path:
    """Pick the scan-dir path. Same-day re-runs auto-suffix with -002, -003, etc.

    `.scratch/security-scan-YYYY-MM-DD/`          (first run today)
    `.scratch/security-scan-YYYY-MM-DD-002/`      (second run)
    `.scratch/security-scan-YYYY-MM-DD-NNN/`      (later runs)

    User can always override with `--out=<dir>`.
    """
    if out:
        return Path(out).expanduser().resolve()
    root = harness_root_default()
    today = dt.datetime.now(tz=dt.UTC).strftime("%Y-%m-%d")
    scratch = root / ".scratch"
    base = scratch / f"security-scan-{today}"
    if not base.exists():
        return base.resolve()
    max_suffix = 999
    for n in range(2, max_suffix + 1):
        candidate = scratch / f"security-scan-{today}-{n:03d}"
        if not candidate.exists():
            return candidate.resolve()
    msg = f"too many security-scan dirs for {today} in {scratch} (exhausted -{max_suffix:03d}); clean up old ones"
    raise RuntimeError(msg)


def _ensure_dir_layout(scan_dir: Path) -> None:
    for sub in (
        "raw/osv-scanner",
        "raw/pip-audit",
        "raw/trivy",
        "raw/grype",
        "normalized",
        "merged",
    ):
        (scan_dir / sub).mkdir(parents=True, exist_ok=True)
    confs = scan_dir / "confirmations.jsonl"
    if not confs.exists():
        confs.touch()


def _capture_version(*argv: str, strip_prefix: str = "") -> str:
    try:
        result = subprocess.run(argv, capture_output=True, text=True, check=False, timeout=10)  # noqa: S603  # internal tooling: argv built from fixed scanner names
    except subprocess.SubprocessError, FileNotFoundError:
        return "(unknown)"
    text = (result.stdout or result.stderr or "").strip()
    first_line = text.splitlines()[0] if text else ""
    if strip_prefix and first_line.startswith(strip_prefix):
        first_line = first_line[len(strip_prefix) :].strip()
    return first_line or "(unknown)"


def _scanner_versions() -> ScannerVersions:
    return ScannerVersions(
        osv_scanner=_capture_version("osv-scanner", "--version", strip_prefix="osv-scanner version:"),
        pip_audit=_capture_version("pip-audit", "--version", strip_prefix="pip-audit"),
        trivy=_capture_version("trivy", "--version", strip_prefix="Version:"),
        grype=_capture_version("grype", "--version", strip_prefix="grype"),
    )


def _tools_manifest(
    scanned_projects: list[str],
    versions: ScannerVersions,
    project_scanners: dict[str, list[ScannerName]],
) -> ToolsManifest:
    tool_catalog = {
        "osv-scanner": ToolEntry(
            version=versions.osv_scanner,
            vuln_db="OSV.dev (GHSA + PYSEC + CVE + RUSTSEC + GO + OSV-Malicious)",
            ecosystem="multi (PyPI, npm, Go, Maven, Cargo, …)",
        ),
        "pip-audit": ToolEntry(
            version=versions.pip_audit,
            vuln_db="PyPI Advisory Database + OSV.dev",
            ecosystem="PyPI",
        ),
        "trivy": ToolEntry(
            version=versions.trivy,
            vuln_db="Aqua vulnerability DB (GHSA + CVE; distinct ingestion path)",
            ecosystem="multi + container + IaC",
        ),
        "grype": ToolEntry(
            version=versions.grype,
            vuln_db="Anchore Vulnerability Feed (NVD + GHSA + distro)",
            ecosystem="multi",
        ),
    }
    scanners_run = {scanner for scanners in project_scanners.values() for scanner in scanners}
    return ToolsManifest(
        scanned_at=dt.datetime.now(tz=dt.UTC).strftime("%Y-%m-%dT%H:%M:%S%z"),
        harness_root=str(harness_root_default()),
        tools={scanner: tool_catalog[scanner] for scanner in SCANNERS if scanner in scanners_run},
        projects_scanned=scanned_projects,
        project_scanners=project_scanners,
    )


# ─── per-scanner invocations ────────────────────────────────────────


def _run_capture(argv: list[str], json_out: Path, err_out: Path) -> int:
    """Run argv; save stdout to json_out, stderr to err_out."""
    json_out.unlink(missing_ok=True)
    err_out.unlink(missing_ok=True)
    try:
        result = subprocess.run(argv, capture_output=True, text=True, check=False)  # noqa: S603  # internal tooling: argv built from fixed scanner commands + trusted paths
    except (subprocess.SubprocessError, FileNotFoundError) as exc:
        err_out.write_text(f"failed to invoke {' '.join(argv)}: {exc}\n")
        return 127
    json_out.write_text(result.stdout or "")
    err_out.write_text(result.stderr or "")
    return result.returncode


def _run_stderr_only(argv: list[str], json_out: Path, err_out: Path) -> int:
    """Run argv (which writes its own JSON via `-o`); only capture stderr."""
    json_out.unlink(missing_ok=True)
    err_out.unlink(missing_ok=True)
    try:
        result = subprocess.run(argv, capture_output=True, text=True, check=False)  # noqa: S603  # internal tooling: argv built from fixed scanner commands + trusted paths
    except (subprocess.SubprocessError, FileNotFoundError) as exc:
        err_out.write_text(f"failed to invoke {' '.join(argv)}: {exc}\n")
        return 127
    err_out.write_text(result.stderr or "")
    return result.returncode


def _require_json_output(scanner: str, json_out: Path, err_out: Path, required_key: str) -> None:
    """Reject missing, malformed, or wrong-schema output instead of reporting zero."""
    try:
        payload = json.loads(json_out.read_text())
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        msg = f"{scanner} did not produce valid JSON; inspect {err_out}: {exc}"
        raise RuntimeError(msg) from exc
    schema_valid = isinstance(payload, dict) and isinstance(payload.get(required_key), list)
    if not schema_valid:
        msg = f"{scanner} JSON lacks expected list field {required_key!r}; inspect {err_out}"
        raise RuntimeError(msg)


def _run_osv(project: str, worktree: Path, scan_dir: Path) -> None:
    out = scan_dir / "raw/osv-scanner" / f"{project}.json"
    err = scan_dir / "raw/osv-scanner" / f"{project}.stderr"
    _run_capture(
        [
            "osv-scanner",
            "scan",
            "source",
            "--recursive",
            "--no-ignore",
            "--include-git-root",
            "--format=json",
            str(worktree),
        ],
        out,
        err,
    )
    _require_json_output("osv-scanner", out, err, "results")


def _run_trivy(project: str, worktree: Path, scan_dir: Path) -> None:
    out = scan_dir / "raw/trivy" / f"{project}.json"
    err = scan_dir / "raw/trivy" / f"{project}.stderr"
    # trivy writes JSON directly via `-o`; we only capture stderr.
    # `--skip-dirs '**/.venv*,...'` catches `.venv.uv-managed.bak` etc.
    _run_stderr_only(
        [
            "trivy",
            "fs",
            "--format",
            "json",
            "--quiet",
            "--scanners",
            "vuln",
            "--skip-dirs",
            "**/.venv*,**/node_modules,**/.cache,**/.git",
            "-o",
            str(out),
            str(worktree),
        ],
        out,
        err,
    )
    _require_json_output("trivy", out, err, "Results")


def _run_grype(project: str, worktree: Path, scan_dir: Path) -> None:
    out = scan_dir / "raw/grype" / f"{project}.json"
    err = scan_dir / "raw/grype" / f"{project}.stderr"
    _run_capture(
        [
            "grype",
            "--output",
            "json",
            "--quiet",
            "--exclude",
            "**/.venv*/**",
            "--exclude",
            "**/node_modules/**",
            "--exclude",
            "**/.cache/**",
            "--exclude",
            "**/.git/**",
            f"dir:{worktree}",
        ],
        out,
        err,
    )
    _require_json_output("grype", out, err, "matches")


def _uv_export(worktree: Path) -> Path | None:
    """Export uv.lock to a temp requirements.txt for pip-audit.

    Returns None when uv.lock is a near-empty stub (no real deps), in
    which case the project's loose `requirements*.txt` are the truth.
    """
    lock = worktree / "uv.lock"
    if not lock.exists():
        return None
    pkg_count = sum(1 for line in lock.read_text().splitlines() if line.startswith("name = "))
    if pkg_count <= 1:
        return None
    tmp = Path(tempfile.mkdtemp(prefix="uv-export-")) / "uvlock.txt"
    try:
        subprocess.run(  # noqa: S603  # internal tooling: trusted worktree path
            [  # noqa: S607  # `uv` resolved from dev-shell PATH
                "uv",
                "export",
                "--project",
                str(worktree),
                "--format",
                "requirements-txt",
                "--no-hashes",
                "--no-emit-project",
                "--no-annotate",
                "-o",
                str(tmp),
            ],
            capture_output=True,
            text=True,
            check=True,
            timeout=60,
        )
    except subprocess.SubprocessError, FileNotFoundError:
        return None
    return tmp if tmp.exists() else None


def _dep_targets(project: str, worktree: Path) -> list[DepTarget]:
    """Enumerate (tag, path) pairs for pip-audit per project."""
    out: list[DepTarget] = []
    exported = _uv_export(worktree)
    if exported is not None:
        out.append(DepTarget(tag=f"{project}__uvlock", path=exported))
    req_re = re.compile(r"requirements([_-].*)?\.txt$")
    for p in worktree.rglob("*"):
        if not p.is_file() or not req_re.search(p.name):
            continue
        rel = p.relative_to(worktree)
        if any(part.startswith((".venv", "node_modules", ".cache")) for part in rel.parts):
            continue
        tag = f"{project}__" + str(rel).replace("/", "_").replace(".txt", "")
        out.append(DepTarget(tag=tag, path=p))
    return out


def _run_pip_audit(target: DepTarget, scan_dir: Path) -> None:
    out = scan_dir / "raw/pip-audit" / f"{target.tag}.json"
    err = scan_dir / "raw/pip-audit" / f"{target.tag}.stderr"
    # pip-audit writes JSON directly via `-o`; we only capture stderr.
    _run_stderr_only(
        [
            "pip-audit",
            "--format=json",
            "--disable-pip",
            "--no-deps",
            "--vulnerability-service=osv",
            "-r",
            str(target.path),
            "-o",
            str(out),
        ],
        out,
        err,
    )
    _require_json_output("pip-audit", out, err, "dependencies")


# ─── orchestration ──────────────────────────────────────────────────


def _print_discovery_table(entries: list[ProjectEntry]) -> None:
    table = Table(title="Candidate projects for scan", show_lines=False)
    table.add_column("Project", style="bold")
    table.add_column("Branch")
    table.add_column("Languages")
    table.add_column("Dep files", justify="right")
    table.add_column("Status")
    for e in entries:
        if not e.scannable:
            table.add_row(
                e.project,
                "-",
                "-",
                "-",
                f"[dim]{e.error or 'no scannable deps'}[/dim]",
            )
            continue
        table.add_row(
            e.project,
            e.branch or "-",
            ",".join(e.languages),
            str(e.deps.total()),
            "[green]scannable[/green]",
        )
    console.print(table)


def _scan_one(
    project: ProjectEntry,
    scan_dir: Path,
    progress: Progress,
    parent_task: TaskID,
) -> list[ScannerName]:
    if not project.scannable:
        progress.advance(parent_task)
        return []
    name = project.project
    worktree = harness_root_default() / project.worktree
    task = progress.add_task(f"[cyan]{name}", total=4)

    _run_osv(name, worktree, scan_dir)
    progress.update(task, advance=1, description=f"[cyan]{name} · osv-scanner")
    _run_trivy(name, worktree, scan_dir)
    progress.update(task, advance=1, description=f"[cyan]{name} · trivy")
    _run_grype(name, worktree, scan_dir)
    progress.update(task, advance=1, description=f"[cyan]{name} · grype")
    dep_targets = _dep_targets(name, worktree)
    for tgt in dep_targets:
        _run_pip_audit(tgt, scan_dir)
    pip_status = "[green]done" if dep_targets else "[dim]not applicable"
    progress.update(task, advance=1, description=f"[cyan]{name} · pip-audit {pip_status}")
    progress.remove_task(task)
    progress.advance(parent_task)
    ran: set[ScannerName] = {"osv-scanner", "trivy", "grype"}
    if dep_targets:
        ran.add("pip-audit")
    return [scanner for scanner in SCANNERS if scanner in ran]


def _print_run_summary(scan_dir: Path) -> None:
    """Rich summary printed after merge + render."""
    unified_path = scan_dir / "merged/unified.jsonl"
    if not unified_path.exists():
        return
    rows = load_jsonl(unified_path, UnifiedRow)

    sev_counts: dict[str, int] = {}
    by_proj: dict[str, dict[str, int]] = {}
    for r in rows:
        sev_counts[r.severity] = sev_counts.get(r.severity, 0) + 1
        proj_counts = by_proj.setdefault(r.project, {})
        proj_counts[r.severity] = proj_counts.get(r.severity, 0) + 1

    summary = Table(title="Scan complete — severity distribution", show_lines=False)
    summary.add_column("Severity")
    summary.add_column("Count", justify="right")
    sev_color = {
        "CRITICAL": "red",
        "HIGH": "yellow",
        "MODERATE": "blue",
        "LOW": "green",
        "UNKNOWN": "dim",
    }
    for sev in ("CRITICAL", "HIGH", "MODERATE", "LOW", "UNKNOWN"):
        if sev_counts.get(sev):
            color = sev_color.get(sev, "white")
            summary.add_row(f"[{color}]{sev}[/{color}]", str(sev_counts[sev]))
    console.print(summary)

    by_proj_table = Table(title="By project", show_lines=False)
    by_proj_table.add_column("Project", style="bold")
    by_proj_table.add_column("CRIT", justify="right", style="red")
    by_proj_table.add_column("HIGH", justify="right", style="yellow")
    by_proj_table.add_column("MOD", justify="right", style="blue")
    by_proj_table.add_column("LOW", justify="right", style="green")
    by_proj_table.add_column("UNK", justify="right", style="dim")
    by_proj_table.add_column("Total", justify="right")
    for proj in sorted(by_proj):
        counts = by_proj[proj]
        by_proj_table.add_row(
            proj,
            str(counts.get("CRITICAL", 0)),
            str(counts.get("HIGH", 0)),
            str(counts.get("MODERATE", 0)),
            str(counts.get("LOW", 0)),
            str(counts.get("UNKNOWN", 0)),
            str(sum(counts.values())),
        )
    console.print(by_proj_table)


# ─── subcommands ────────────────────────────────────────────────────


@app.command()
def discover_(
    projects: Annotated[
        str,
        typer.Option("--projects", help="comma-separated subset (default: all registered)"),
    ] = "",
    output_format: Annotated[
        str,
        typer.Option("--format", help="`table` (default) or `json`"),
    ] = "table",
) -> None:
    """List candidate projects for scanning."""
    entries = discover(harness_root_default(), _parse_projects(projects) or None)
    if output_format == "json":
        payload = {
            "harness_root": str(harness_root_default()),
            "projects": [e.model_dump() for e in entries],
        }
        json.dump(payload, sys.stdout, indent=2, default=str)
        sys.stdout.write("\n")
    elif output_format == "table":
        _print_discovery_table(entries)
    else:
        msg = f"invalid --format={output_format!r}; expected `table` or `json`"
        raise typer.BadParameter(msg)


# Typer renames `discover_` → `discover-`; keep the user-facing name clean.
discover_.__name__ = "discover"
app.command(name="discover")(discover_)


@app.command()
def run(
    projects: Annotated[
        str,
        typer.Option(
            "--projects",
            help="comma-separated subset (required unless --all-projects)",
        ),
    ] = "",
    all_projects: Annotated[  # noqa: FBT002  # typer --all-projects flag; bool default is the CLI default
        bool,
        typer.Option("--all-projects", help="scan every project with scannable deps"),
    ] = False,
    out: Annotated[
        str,
        typer.Option(
            "--out",
            help="output dir (default: .scratch/security-scan-YYYY-MM-DD)",
        ),
    ] = "",
) -> None:
    """Run the 4-scanner pipeline + normalize + merge. Stops before rendering.

    Rendering is a separate step (`report.py render`) — keep this CLI
    focused on producing raw + normalized + merged data only. Multiple
    agents can then run `report analyze` against the same scan dir and
    finally `report render` to materialize REPORT/XRAY.
    """
    selected_names = _parse_projects(projects)
    if not selected_names and not all_projects:
        console.print("[red]missing --projects=A,B,C or --all-projects[/red]")
        console.print("Try `scan.py discover` first to see candidates.")
        raise typer.Exit(code=2)

    all_entries = discover(harness_root_default(), selected_names or None)
    scannable = [e for e in all_entries if e.scannable]
    if not scannable:
        console.print("[red]no scannable projects found[/red]")
        raise typer.Exit(code=1)

    selected = scannable if all_projects else [e for e in scannable if e.project in selected_names]
    if not selected:
        console.print("[red]none of the named projects are scannable[/red]")
        raise typer.Exit(code=1)

    _require_tools()
    scan_dir = _scan_dir_default(out or None)
    _ensure_dir_layout(scan_dir)

    console.print(
        Panel.fit(
            f"[bold]scan-dir[/bold]: {scan_dir}\n[bold]projects[/bold]: " + ", ".join(e.project for e in selected),
            title="security-scan",
            border_style="cyan",
        ),
    )

    versions = _scanner_versions()
    project_scanners: dict[str, list[ScannerName]] = {}
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        TimeElapsedColumn(),
        console=console,
        transient=False,
    ) as progress:
        parent = progress.add_task("[bold]scanning…", total=len(selected))
        for entry in selected:
            project_scanners[entry.project] = _scan_one(entry, scan_dir, progress, parent)
        progress.update(parent, description="[bold green]scanners done")

    manifest = _tools_manifest([e.project for e in selected], versions, project_scanners)
    (scan_dir / "tools.json").write_text(manifest.model_dump_json(indent=2) + "\n")

    console.print("\n[bold]=> normalize[/bold]")
    norm_stats = normalize(scan_dir)
    for s, n in norm_stats.per_scanner.items():
        console.print(f"  {s:<14} +{n:>4d} rows")
    console.print(f"  total {norm_stats.total}")

    console.print("\n[bold]=> merge[/bold]")
    merge_stats = merge(scan_dir)
    console.print(f"  {merge_stats.raw_count} raw → {merge_stats.unified_count} unified")

    console.print()
    _print_run_summary(scan_dir)

    report_render = f"security-report render --scan-dir={scan_dir}"
    report_pending = f"security-report pending --scan-dir={scan_dir} --format=table"
    console.print(
        Panel.fit(
            "[bold]Raw data ready.[/bold] No reports rendered yet — that's a separate step.\n\n"
            f"To see findings needing analysis:\n  [cyan]{report_pending}[/cyan]\n\n"
            f"To render REPORT.{{md,html}} + XRAY.{{md,html}}:\n  [cyan]{report_render}[/cyan]",
            title="scan done",
            border_style="green",
        ),
    )


# Reserved import — Finding is exposed for downstream tools that may chain
# normalize() output into custom analysis. Quiets unused-import linting.
_ = Finding


if __name__ == "__main__":
    app()
