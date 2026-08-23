from datetime import UTC, datetime
from pathlib import Path

import pytest

from agent_tools.memory_sync.facts import RawFact
from agent_tools.memory_sync.notes import (
    DestinationMismatchError,
    build_notes,
    notes_dir,
    returns_to_its_own_store,
    write_notes,
)

MOMENT = datetime(2026, 8, 6, 17, 30, 0, tzinfo=UTC)
ORDINARY = Path("/stores/ordinary")
DEEPSEEK = Path("/stores/deepseek")


def raw(name: str, source: str = "", source_home: str = "") -> RawFact:
    metadata = {}
    if source:
        metadata["source"] = source
    if source_home:
        metadata["source_home"] = source_home
    return RawFact(
        path=Path(f"/nonexistent/store/{name}.md"),
        name=name,
        description=f"description for {name}",
        metadata=metadata,
        body="Do the thing.",
    )


def test_a_fact_never_returns_to_the_store_it_came_from():
    fact = raw("from-ordinary", "codex", str(ORDINARY))
    assert returns_to_its_own_store(fact, ORDINARY)
    report = build_notes([fact], ORDINARY, now=MOMENT)
    assert report.skipped_same_store == 1
    assert report.notes == []


def test_one_codex_home_may_sync_into_another():
    # The case a runtime-name check could not express: both stores are Codex.
    fact = raw("from-ordinary", "codex", str(ORDINARY))
    assert not returns_to_its_own_store(fact, DEEPSEEK)
    report = build_notes([fact], DEEPSEEK, now=MOMENT)
    assert report.skipped_same_store == 0
    assert [note.filename for note in report.notes] == ["20260806-173000-from-ordinary.md"]


def test_store_comparison_ignores_path_spelling():
    fact = raw("from-ordinary", "codex", f"{ORDINARY}/")
    assert returns_to_its_own_store(fact, Path(f"{ORDINARY}/./"))


def test_a_fact_from_another_runtime_flows_through():
    fact = raw("from-claude", "claude", "/stores/claude")
    report = build_notes([fact], ORDINARY, now=MOMENT)
    assert [note.filename for note in report.notes] == ["20260806-173000-from-claude.md"]


def test_unknown_origin_falls_back_to_the_runtime_name():
    # Hand-authored files carry no provenance; stay conservative for Codex.
    assert returns_to_its_own_store(raw("hand", "codex"), ORDINARY)
    assert not returns_to_its_own_store(raw("hand", "shared"), ORDINARY)
    assert not returns_to_its_own_store(raw("hand"), ORDINARY)


def test_note_records_its_origin_as_information():
    report = build_notes([raw("from-shared", "shared")], ORDINARY, now=MOMENT)
    rendered = report.notes[0].render()
    assert rendered.startswith("# From shared")
    assert "description for from-shared" in rendered
    assert "Recorded by shared" in rendered


def test_write_notes_creates_the_extension_directory(tmp_path):
    report = build_notes([raw("from-shared", "shared")], tmp_path, now=MOMENT)
    write_notes(tmp_path, report)
    written = notes_dir(tmp_path) / "20260806-173000-from-shared.md"
    assert written.is_file()
    assert report.written == [written]


def test_write_notes_never_replaces_an_existing_note(tmp_path):
    directory = notes_dir(tmp_path)
    directory.mkdir(parents=True)
    (directory / "20260101-000000-from-shared.md").write_text("original\n", encoding="utf-8")
    report = build_notes([raw("from-shared", "shared")], tmp_path, now=MOMENT)
    write_notes(tmp_path, report)
    assert report.skipped_existing == 1
    assert report.written == []
    assert (directory / "20260101-000000-from-shared.md").read_text(encoding="utf-8") == "original\n"


def test_writing_a_report_to_a_different_store_is_refused(tmp_path):
    # The guard only works if the filter and the destination cannot drift apart.
    report = build_notes([raw("from-shared", "shared")], tmp_path / "intended", now=MOMENT)
    with pytest.raises(DestinationMismatchError, match="rebuild them for the destination"):
        write_notes(tmp_path / "elsewhere", report)
    assert not (tmp_path / "elsewhere").exists()


def test_build_notes_requires_a_destination():
    # A caller cannot reach the writer without stating where notes are going.
    # The type checker rejects this too; the runtime check is the second layer.
    with pytest.raises(TypeError):
        build_notes([raw("from-shared", "shared")])  # ty: ignore[missing-argument]
