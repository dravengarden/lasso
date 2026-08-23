"""Emit Codex ad-hoc notes from another runtime's facts.

Codex compiles `MEMORY.md` and `memory_summary.md` from rollout summaries and
ad-hoc notes, so writing to either artifact would be overwritten on the next
consolidation. `extensions/ad_hoc/notes/` is the supported entry point: its own
instructions state that notes are authoritative for adding, editing, and
deleting memories, that a note is never deleted, and that note content is
information rather than instructions.

A fact never returns to the store it came from. Without that rule the pipeline
forms a loop in which Codex compiles a fact, the fact is extracted, the note is
consolidated back, and each pass rewrites the statement until it no longer
resembles what was learned.

The rule compares stores, not runtime names. Two Codex homes exist on this
machine — the ordinary one and the DeepSeek provider's — and copying a fact
from one to the other is a legitimate sync, while copying it back into its own
store is the loop. A runtime-name check cannot tell those apart.

Every entry point here requires the destination, so a caller cannot reach the
writer without stating where the notes are going.
"""

from __future__ import annotations

from datetime import UTC, datetime
from pathlib import Path

from pydantic import BaseModel, Field

from agent_tools.memory_sync.facts import RawFact, canonical_home

NOTES_RELATIVE = Path("extensions/ad_hoc/notes")

# A fact from an unknown store is only refused when it names the same runtime as
# the destination. Hand-authored files carry no provenance, and refusing every
# one of them would make the reverse direction useless.
UNKNOWN_ORIGIN_RUNTIME = "codex"


class Note(BaseModel):
    """One ad-hoc note ready to be written into the Codex memory extension."""

    filename: str
    title: str
    body: str

    def render(self) -> str:
        """Render the note as Codex reads it: a heading and information bullets."""
        return f"# {self.title}\n\n{self.body.strip()}\n"


class NotesReport(BaseModel):
    """Outcome of one reverse-direction run, bound to one destination.

    The destination is recorded rather than passed again at write time, so the
    notes that were filtered for one store cannot be written into another.
    """

    target_home: str
    notes: list[Note] = Field(default_factory=list)
    skipped_same_store: int = 0
    skipped_existing: int = 0
    written: list[Path] = Field(default_factory=list)


def _slug(value: str) -> str:
    kept = [character if character.isalnum() else "-" for character in value.lower()]
    return "-".join(part for part in "".join(kept).split("-") if part)[:60]


def build_note(fact: RawFact, *, now: datetime) -> Note:
    """Build one note from a fact, keeping its origin visible to Codex."""
    stamp = now.strftime("%Y%m%d-%H%M%S")
    lines = [f"- {fact.description.strip()}"] if fact.description.strip() else []
    body = fact.body.strip()
    if body:
        lines.append("")
        lines.append(body)
    origin = fact.source or "another runtime"
    lines.append("")
    lines.append(f"- Recorded by {origin}; translated by the Lasso Memory plugin.")
    return Note(
        filename=f"{stamp}-{_slug(fact.name)}.md",
        title=fact.name.replace("-", " ").capitalize(),
        body="\n".join(lines),
    )


def returns_to_its_own_store(fact: RawFact, target_home: Path) -> bool:
    """Return whether writing this fact into `target_home` would close the loop.

    A fact that records the store it came from is compared against the
    destination. A fact with no recorded store is refused only when it claims
    the destination's runtime, which keeps the old conservative behavior for
    files this plugin did not write.
    """
    target = canonical_home(target_home)
    if fact.source_home:
        return canonical_home(fact.source_home) == target
    return fact.source == UNKNOWN_ORIGIN_RUNTIME


def build_notes(facts: list[RawFact], target_home: Path, *, now: datetime | None = None) -> NotesReport:
    """Build notes for every fact that did not come from `target_home`.

    `target_home` is required: the filter and the destination must be decided
    together, or the guard is trivially bypassed by building for one store and
    writing to another.
    """
    moment = now or datetime.now(UTC)
    report = NotesReport(target_home=canonical_home(target_home))
    for fact in facts:
        if returns_to_its_own_store(fact, target_home):
            report.skipped_same_store += 1
            continue
        report.notes.append(build_note(fact, now=moment))
    return report


def notes_dir(codex_home: Path) -> Path:
    """Return the Codex ad-hoc notes directory for a Codex home directory."""
    return codex_home / "memories" / NOTES_RELATIVE


class DestinationMismatchError(RuntimeError):
    """Raised when a report is written somewhere it was not filtered for."""


def write_notes(codex_home: Path, report: NotesReport) -> NotesReport:
    """Write notes that do not already exist, never replacing an existing one.

    The destination is re-checked against the one the report was built for.
    That check is redundant on every current path, which is the point: it turns
    a future refactor that separates the two into an error instead of a silent
    loop.
    """
    if canonical_home(codex_home) != report.target_home:
        message = (
            f"notes were filtered for {report.target_home} but would be written to "
            f"{canonical_home(codex_home)}; rebuild them for the destination"
        )
        raise DestinationMismatchError(message)
    directory = notes_dir(codex_home)
    directory.mkdir(parents=True, exist_ok=True)
    existing = {path.name.split("-", 2)[-1] for path in directory.glob("*.md")}
    for note in report.notes:
        if note.filename.split("-", 2)[-1] in existing:
            report.skipped_existing += 1
            continue
        path = directory / note.filename
        path.write_text(note.render(), encoding="utf-8")
        report.written.append(path)
    return report
