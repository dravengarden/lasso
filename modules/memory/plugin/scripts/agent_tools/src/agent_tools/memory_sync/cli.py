"""Command line entry point for Codex memory translation and follower-store audits."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Annotated

import typer
from rich.console import Console
from rich.table import Table

from agent_tools.memory_sync import codex, extract, gateway, notes, stale
from agent_tools.memory_sync.facts import read_facts, write_facts

app = typer.Typer(
    help="Translate durable facts between Codex homes and audit follower memory stores read-only.",
    no_args_is_help=True,
    add_completion=False,
)
console = Console()
errors = Console(stderr=True)

DEFAULT_CODEX_HOME = Path.home() / ".codex"
DEFAULT_CLAUDE_HOME = Path.home() / ".claude"
DEFAULT_GROK_HOME = Path.home() / ".grok"
DEFAULT_STAGING = Path.home() / ".cache" / "lasso-memory" / "facts"

CodexHome = Annotated[Path, typer.Option("--codex-home", help="Codex home directory.")]
Staging = Annotated[Path, typer.Option("--staging", help="Directory receiving proposed fact files.")]
JsonOut = Annotated[bool, typer.Option("--json", help="Emit JSON on stdout instead of a table.")]


@app.command()
def scan(
    codex_home: CodexHome = DEFAULT_CODEX_HOME,
    *,
    json_out: JsonOut = False,
) -> None:
    """Parse the Codex memory artifact and report its structure. No model call."""
    groups, revision = codex.load(codex_home)
    if not groups:
        errors.print(f"[yellow]no Codex memory found under {codex.memory_path(codex_home)}[/yellow]")
        raise typer.Exit(1)
    sections: dict[str, int] = {}
    for group in groups:
        for bullet in group.bullets:
            sections[bullet.section] = sections.get(bullet.section, 0) + 1
    if json_out:
        console.print_json(
            json.dumps(
                {
                    "revision": revision,
                    "groups": len(groups),
                    "bullets": sum(sections.values()),
                    "sections": sections,
                    "titles": [group.title for group in groups],
                }
            )
        )
        return
    table = Table(title=f"Codex memory at {revision[:12] or 'unversioned'}")
    table.add_column("section")
    table.add_column("bullets", justify="right")
    for section, count in sorted(sections.items()):
        table.add_row(section, str(count))
    table.add_row("task groups", str(len(groups)))
    console.print(table)


@app.command("stale")
def stale_command(
    claude_home: Annotated[Path, typer.Option("--claude-home")] = DEFAULT_CLAUDE_HOME,
    grok_home: Annotated[Path, typer.Option("--grok-home")] = DEFAULT_GROK_HOME,
    staging: Staging = DEFAULT_STAGING,
    *,
    json_out: JsonOut = False,
) -> None:
    """Report facts referencing paths that no longer exist. Never deletes anything."""
    locations = [
        *stale.claude_memory_dirs(claude_home),
        *stale.grok_memory_locations(grok_home),
        staging,
    ]
    findings = stale.scan(locations)
    if json_out:
        console.print_json(json.dumps([finding.model_dump(mode="json") for finding in findings]))
        return
    if not findings:
        console.print(f"[green]no stale path references across {len(locations)} locations[/green]")
        return
    table = Table(title="Stale path references (review; nothing was removed)")
    table.add_column("fact")
    table.add_column("source")
    table.add_column("missing")
    for finding in findings:
        table.add_row(finding.name, finding.source or "unknown", "\n".join(finding.missing))
    console.print(table)


@app.command("extract")
def extract_command(
    codex_home: CodexHome = DEFAULT_CODEX_HOME,
    staging: Staging = DEFAULT_STAGING,
    guidance: Annotated[list[Path] | None, typer.Option("--guidance", help="Guidance files to suppress.")] = None,
    endpoint: Annotated[str, typer.Option("--endpoint", help="Loopback model gateway.")] = gateway.DEFAULT_ENDPOINT,
    model: Annotated[str, typer.Option("--model")] = gateway.DEFAULT_MODEL,
    limit: Annotated[int, typer.Option("--limit", help="Process only the first N task groups.")] = 0,
    max_tokens: Annotated[
        int,
        typer.Option("--max-tokens", help="Output budget per call; DeepSeek thinking is billed against it."),
    ] = gateway.DEFAULT_MAX_TOKENS,
    *,
    keep_volatile: Annotated[
        bool,
        typer.Option("--keep-volatile", help="Keep facts pinning a port, revision, version, or source symbol."),
    ] = False,
    write: Annotated[bool, typer.Option("--write", help="Write fact files into the staging directory.")] = False,
    json_out: JsonOut = False,
) -> None:
    """Extract durable facts from Codex memory through the DeepSeek gateway.

    Writes nothing unless --write is passed, and never writes into a live
    runtime memory store: the staging directory is the review surface.
    """
    groups, revision = codex.load(codex_home)
    if not groups:
        errors.print(f"[red]no Codex memory found under {codex.memory_path(codex_home)}[/red]")
        raise typer.Exit(1)
    report = extract.extract(
        groups,
        revision,
        extract.read_guidance(list(guidance or [])),
        source_home=codex_home,
        endpoint=endpoint,
        model=model,
        limit=limit,
        max_tokens=max_tokens,
        keep_volatile=keep_volatile,
    )
    if write and report.facts:
        write_facts(staging, report.facts)
    if json_out:
        console.print_json(report.model_dump_json())
    else:
        _print_report(report, staging, write=write)
    if report.failures and not report.facts:
        raise typer.Exit(1)


def _print_report(report: extract.Report, staging: Path, *, write: bool) -> None:
    table = Table(title=f"Extraction from Codex memory {report.revision[:12] or 'unversioned'}")
    table.add_column("metric")
    table.add_column("value", justify="right")
    table.add_row("task groups", str(report.groups))
    table.add_row("input bullets", str(report.bullets))
    table.add_row("model proposals", str(report.proposed))
    table.add_row("dropped: not durable", str(report.dropped_not_durable))
    table.add_row("dropped: already in guidance", str(report.dropped_covered))
    table.add_row("dropped: volatile detail", str(report.dropped_volatile))
    table.add_row("dropped: invalid shape", str(report.dropped_invalid))
    table.add_row("merged duplicates", str(report.merged))
    table.add_row("conflicts recorded", str(len(report.conflicts)))
    table.add_row("facts kept", str(len(report.facts)))
    cache = report.usage.cache_read_tokens
    table.add_row("input tokens", str(report.usage.input_tokens))
    table.add_row("cache read tokens", "not reported" if cache is None else str(cache))
    table.add_row("output tokens", str(report.usage.output_tokens))
    console.print(table)
    for conflict in report.conflicts:
        errors.print(
            f"[yellow]conflict[/yellow] {conflict.name}: kept {conflict.kept_from}, dropped {conflict.dropped_from}"
        )
    for failure in report.failures:
        errors.print(f"[red]failed[/red] {failure}")
    if write:
        console.print(f"[green]wrote {len(report.facts)} facts to {staging}[/green]")
    else:
        console.print("[dim]dry run; pass --write to stage the facts[/dim]")


@app.command("notes")
def notes_command(
    staging: Staging = DEFAULT_STAGING,
    codex_home: CodexHome = DEFAULT_CODEX_HOME,
    *,
    write: Annotated[bool, typer.Option("--write", help="Write notes into the Codex ad-hoc extension.")] = False,
    json_out: JsonOut = False,
) -> None:
    """Turn non-Codex facts into Codex ad-hoc notes.

    Facts that Codex produced are refused, which is what keeps the two
    directions from forming a rewrite loop.
    """
    facts = read_facts(staging)
    report = notes.build_notes(facts, codex_home)
    if write:
        notes.write_notes(codex_home, report)
    if json_out:
        console.print_json(report.model_dump_json())
        return
    table = Table(title="Codex ad-hoc notes")
    table.add_column("metric")
    table.add_column("value", justify="right")
    table.add_row("facts read", str(len(facts)))
    table.add_row("refused: came from this store", str(report.skipped_same_store))
    table.add_row("skipped: note already present", str(report.skipped_existing))
    table.add_row("notes written", str(len(report.written)))
    console.print(table)
    if not write:
        console.print("[dim]dry run; pass --write to create the notes[/dim]")


if __name__ == "__main__":  # pragma: no cover
    app()
