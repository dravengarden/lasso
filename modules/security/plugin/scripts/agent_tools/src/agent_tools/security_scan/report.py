r"""Phase 2 of the `security-scan` skill — multi-agent analysis + report.

Three subcommands:

  pending  --scan-dir DIR        list findings that still need agent analysis
  analyze  --scan-dir DIR ...    append one agent's verdict to confirmations.jsonl
  render   --scan-dir DIR        materialize REPORT.{md,html} + XRAY.{md,html} +
                                 iocs/<campaign-slug>.{md,html}

Independent-review flow:

  scan.py run       …               (produces raw + normalized + merged + tools.json)
  report.py analyze --agent-runtime=codex --agent-model=current-model-id \\
                    --cluster-id=X --project=P --package=PKG --version=V \\
                    --verdict=real …
  report.py render  …               (renders independent verdicts and conflicts)

Invariants:
- `raw/`, `normalized/`, `merged/`, `tools.json` are read-only here
- `confirmations.jsonl` is APPEND-ONLY; corrections via `verdict=superseded`
- Re-running `render` only touches REPORT.{md,html}, XRAY.{md,html}, iocs/*

Two separate reports are produced:
- **REPORT.md / report.html** — traditional CVE/GHSA findings only
- **XRAY.md  / xray.html**    — supply-chain attack matches (IOC feed) and
                                links to per-campaign full IOC listings
"""

from __future__ import annotations

import datetime as dt
import json
import sys
from pathlib import Path  # noqa: TC003  # used at runtime in typer Annotated[Path, ...] params
from typing import Annotated

import typer
from rich.console import Console
from rich.table import Table

from .models import (
    SCANNERS,
    SEV_RANK,
    VERDICTS,
    Confirmation,
    UnifiedRow,
    Verdict,
    append_jsonl,
    load_jsonl,
    load_tools_manifest,
)
from .render import (
    render,
)

console = Console()
app = typer.Typer(
    no_args_is_help=True,
    add_completion=False,
    pretty_exceptions_show_locals=False,
    rich_markup_mode="markdown",
    help="Multi-agent analysis + report rendering for the security-scan skill.",
)


# ─── shared helpers ─────────────────────────────────────────────────


def _require_scan_dir(scan_dir: Path) -> Path:
    if not scan_dir.exists():
        console.print(f"[red]scan dir not found:[/red] {scan_dir}")
        raise typer.Exit(code=1)
    unified = scan_dir / "merged" / "unified.jsonl"
    if not unified.exists():
        console.print(
            f"[red]unified findings missing:[/red] {unified}\nRun [bold]security-scan run[/bold] first.",
        )
        raise typer.Exit(code=1)
    return scan_dir.resolve()


def _confirmations_path(scan_dir: Path) -> Path:
    return scan_dir / "confirmations.jsonl"


def _now_iso() -> str:
    return dt.datetime.now(tz=dt.UTC).strftime("%Y-%m-%dT%H:%M:%S%z")


def _parse_verdict(value: str) -> Verdict:
    if value not in VERDICTS:
        msg = f"invalid --verdict={value!r}; valid: {', '.join(VERDICTS)}"
        raise typer.BadParameter(msg)
    match value:
        case "real":
            return "real"
        case "false-positive":
            return "false-positive"
        case "fixed":
            return "fixed"
        case "accepted-risk":
            return "accepted-risk"
        case "superseded":
            return "superseded"
        case _:
            msg = f"unreachable verdict: {value}"
            raise AssertionError(msg)


# ─── pending ────────────────────────────────────────────────────────


def _filter_pending(
    unified: list[UnifiedRow],
    confs: list[Confirmation],
    *,
    min_severity: str,
    include_all: bool,
) -> list[UnifiedRow]:
    """Return the subset of unified rows that still need agent analysis.

    Default rule:
      - severity ≥ `min_severity`, OR
      - scanner_count == 1 (low-confidence), OR
      - not yet analyzed by any agent
    """
    if include_all:
        return list(unified)
    if min_severity not in SEV_RANK:
        msg = f"invalid --severity={min_severity!r}; expected one of {list(SEV_RANK)}"
        raise typer.BadParameter(msg)
    cutoff = SEV_RANK[min_severity]
    conf_keys = {c.key() for c in confs}
    out: list[UnifiedRow] = []
    for r in unified:
        sev_score = SEV_RANK.get(r.severity, SEV_RANK["UNKNOWN"])
        already = r.key() in conf_keys
        if sev_score <= cutoff or r.scanner_count == 1 or not already:
            out.append(r)
    out.sort(key=lambda r: (SEV_RANK[r.severity], r.scanner_count, r.project))
    return out


def _print_pending_table(pending: list[UnifiedRow], scanner_totals: dict[str, int]) -> None:
    table = Table(title=f"Findings pending agent analysis ({len(pending)})", show_lines=False)
    table.add_column("Sev", style="bold")
    table.add_column("Project")
    table.add_column("Package")
    table.add_column("Version")
    table.add_column("Scanners", justify="right")
    table.add_column("Cluster")
    sev_color = {
        "CRITICAL": "red",
        "HIGH": "yellow",
        "MODERATE": "blue",
        "LOW": "green",
        "UNKNOWN": "dim",
    }
    for r in pending:
        c = sev_color.get(r.severity, "white")
        table.add_row(
            f"[{c}]{r.severity}[/{c}]",
            r.project,
            r.package,
            r.version,
            f"{r.scanner_count}/{scanner_totals.get(r.project, len(SCANNERS))}",
            r.cluster_id,
        )
    console.print(table)


@app.command()
def pending(
    scan_dir: Annotated[Path, typer.Option("--scan-dir", help="existing scan directory")],
    severity: Annotated[
        str,
        typer.Option("--severity", help="minimum severity to include"),
    ] = "HIGH",
    include_all: Annotated[  # noqa: FBT002  # typer --all flag; bool default is the CLI default
        bool,
        typer.Option("--all", help="include every finding (ignore the default filter)"),
    ] = False,
    output_format: Annotated[
        str,
        typer.Option("--format", help="`json` (default, one row per line) or `table`"),
    ] = "json",
) -> None:
    """Print findings that still need an agent's analysis."""
    sd = _require_scan_dir(scan_dir)
    unified = load_jsonl(sd / "merged" / "unified.jsonl", UnifiedRow)
    confs = load_jsonl(_confirmations_path(sd), Confirmation)
    tools_manifest = load_tools_manifest(sd / "tools.json")
    rows = _filter_pending(unified, confs, min_severity=severity, include_all=include_all)
    if output_format == "json":
        for row in rows:
            sys.stdout.write(row.model_dump_json() + "\n")
    elif output_format == "table":
        scanner_totals = {
            project: tools_manifest.scanner_total_for(project) for project in {row.project for row in rows}
        }
        _print_pending_table(rows, scanner_totals)
    else:
        msg = f"invalid --format={output_format!r}; expected `json` or `table`"
        raise typer.BadParameter(msg)


# ─── analyze ────────────────────────────────────────────────────────


@app.command()
def analyze(  # noqa: PLR0913  # typer CLI: each arg is a distinct --flag, not refactorable
    scan_dir: Annotated[Path, typer.Option("--scan-dir", help="existing scan directory")],
    cluster_id: Annotated[str, typer.Option("--cluster-id", help="vuln cluster id")],
    project: Annotated[str, typer.Option("--project")],
    package: Annotated[str, typer.Option("--package")],
    version: Annotated[str, typer.Option("--version")],
    agent_runtime: Annotated[
        str,
        typer.Option(
            "--agent-runtime",
            help="review runtime: codex | claude-code | human",
        ),
    ],
    agent_model: Annotated[
        str,
        typer.Option(
            "--agent-model",
            help="actual agent model id, or reviewer name for a human review",
        ),
    ],
    verdict: Annotated[
        str,
        typer.Option(
            "--verdict",
            help="real | false-positive | fixed | accepted-risk | superseded",
        ),
    ],
    fix: Annotated[str, typer.Option("--fix", help="fix recommendation")] = "",
    notes: Annotated[str, typer.Option("--notes", help="free-form context")] = "",
    supersedes: Annotated[
        str,
        typer.Option(
            "--supersedes",
            help="ISO timestamp of prior row this one corrects (required if verdict=superseded)",
        ),
    ] = "",
) -> None:
    """Append one agent's analysis to confirmations.jsonl.

    Multiple agents can analyze the same finding. The renderer marks
    conflicting verdicts so the team can adjudicate.
    """
    sd = _require_scan_dir(scan_dir)
    parsed_verdict = _parse_verdict(verdict)
    if parsed_verdict == "superseded" and not supersedes:
        msg = "--verdict=superseded requires --supersedes=<prior timestamp>"
        raise typer.BadParameter(msg)
    if not agent_runtime or not agent_model:
        msg = (
            "--agent-runtime and --agent-model are required; e.g. --agent-runtime=codex --agent-model=current-model-id"
        )
        raise typer.BadParameter(msg)

    unified = load_jsonl(sd / "merged" / "unified.jsonl", UnifiedRow)
    matches = [
        r
        for r in unified
        if r.cluster_id == cluster_id and r.project == project and r.package == package and r.version == version
    ]
    if not matches:
        nearest = [
            f"  {r.cluster_id}  {r.project}  {r.package}  {r.version}"
            for r in unified
            if r.project == project and r.package == package
        ][:3]
        suggest = "\n".join(nearest) or "  (no matches for this project + package)"
        console.print(
            f"[red]no finding matches:[/red] "
            f"cluster_id={cluster_id} project={project} package={package} version={version}",
        )
        console.print(f"[dim]nearest:[/dim]\n{suggest}")
        raise typer.Exit(code=2)

    row = Confirmation(
        cluster_id=cluster_id,
        project=project,
        package=package,
        version=version,
        agent_runtime=agent_runtime,
        agent_model=agent_model,
        confirmed_at=_now_iso(),
        verdict=parsed_verdict,
        fix=fix,
        notes=notes,
        supersedes=supersedes,
    )
    append_jsonl(_confirmations_path(sd), row)
    console.print(
        f"[green]appended:[/green] {project} {package}@{version} → [bold]{parsed_verdict}[/bold] (by {row.agent_id()})",
    )


# ─── render ─────────────────────────────────────────────────────────


@app.command(name="render")
def render_(
    scan_dir: Annotated[Path, typer.Option("--scan-dir", help="existing scan directory")],
) -> None:
    """Render REPORT.{md,html} + XRAY.{md,html} + iocs/<campaign>.{md,html}.

    Re-run any time confirmations are appended; only the rendered files
    change, never `raw/` / `normalized/` / `merged/`.
    """
    sd = _require_scan_dir(scan_dir)
    result = render(sd)
    console.print(
        f"[green]rendered:[/green] {result.unified_count} findings · {result.confirmations_count} confirmations",
    )
    console.print(f"  REPORT  → {result.md_path}")
    console.print(f"  REPORT  → {result.html_path}")
    if hasattr(result, "xray_md_path") and result.xray_md_path:
        console.print(f"  XRAY    → {result.xray_md_path}")
        console.print(f"  XRAY    → {result.xray_html_path}")


# Quiet unused-import lints for re-exported names.
_ = json


if __name__ == "__main__":
    app()
