from agent_tools.memory_sync.facts import INDEX_LINE, Fact, read_fact, read_facts, valid_name, write_facts

# Reproduces a real Claude Code memory file: a strict subset of the keys this
# plugin stages, with an unknown key Claude tolerates.
CLAUDE_FILE = """\
---
name: review-scope-ignore-lan-security
description: "When reviewing the hawk NixOS host, LAN-internal security exposure is out of scope"
metadata:
  node_type: memory
  type: feedback
  originSessionId: dbb4c196
---

Do not report LAN-internal security exposure on `/etc/nixos`.

**Why:** the host is single-user on a trusted intranet.
"""


def sample_fact(**overrides) -> Fact:
    base = Fact(
        name="fix-confirmed-bugs",
        title="Fix confirmed bugs",
        description='After diagnosis confirms a bug, repair it: "fix 吧" means repair, not explain',
        type="feedback",
        scope="global",
        body="Once a bug is confirmed, make the targeted repair and add regression coverage.",
        why="The user asks for a fix, not an explanation.",
        how_to_apply="Repair, test, deploy when in scope.",
        source="codex",
    )
    return base.model_copy(update=overrides)


def test_render_round_trips_through_the_tolerant_reader(tmp_path):
    fact = sample_fact(source_refs=("Group A",), source_revision="abc123")
    path = tmp_path / "fix-confirmed-bugs.md"
    path.write_text(fact.render(), encoding="utf-8")
    raw = read_fact(path)
    assert raw.name == fact.name
    assert raw.description == fact.description
    assert raw.metadata["type"] == "feedback"
    assert raw.metadata["scope"] == "global"
    assert raw.source == "codex"
    assert "regression coverage" in raw.body
    assert "**Why:**" in raw.body


def test_description_quoting_survives_embedded_quotes(tmp_path):
    fact = sample_fact(description='He said "fix 吧" and meant it')
    path = tmp_path / "quoted.md"
    path.write_text(fact.render(), encoding="utf-8")
    assert read_fact(path).description == 'He said "fix 吧" and meant it'


def test_reads_a_claude_authored_file(tmp_path):
    path = tmp_path / "review-scope-ignore-lan-security.md"
    path.write_text(CLAUDE_FILE, encoding="utf-8")
    raw = read_fact(path)
    assert raw.name == "review-scope-ignore-lan-security"
    assert raw.metadata["type"] == "feedback"
    assert raw.source == ""
    assert "trusted intranet" in raw.body


def test_reads_a_file_without_frontmatter(tmp_path):
    path = tmp_path / "bare-note.md"
    path.write_text("just a body\n", encoding="utf-8")
    raw = read_fact(path)
    assert raw.name == "bare-note"
    assert raw.body == "just a body"


def test_index_line_matches_the_shared_index_pattern():
    line = sample_fact().index_line()
    match = INDEX_LINE.match(line)
    assert match is not None
    assert match.group(1) == "fix-confirmed-bugs"


def test_write_facts_preserves_hand_authored_index_content(tmp_path):
    index = tmp_path / "MEMORY.md"
    tmp_path.mkdir(exist_ok=True)
    index.write_text("# Notes\n\nkept by hand\n", encoding="utf-8")
    write_facts(tmp_path, [sample_fact()])
    content = index.read_text(encoding="utf-8")
    assert "kept by hand" in content
    assert "(fix-confirmed-bugs.md)" in content


def test_write_facts_is_idempotent(tmp_path):
    write_facts(tmp_path, [sample_fact()])
    first = (tmp_path / "MEMORY.md").read_text(encoding="utf-8")
    write_facts(tmp_path, [sample_fact()])
    assert (tmp_path / "MEMORY.md").read_text(encoding="utf-8") == first


def test_read_facts_skips_the_index(tmp_path):
    write_facts(tmp_path, [sample_fact()])
    assert [fact.name for fact in read_facts(tmp_path)] == ["fix-confirmed-bugs"]


def test_digest_tracks_content_not_provenance():
    base = sample_fact()
    assert base.digest == sample_fact(source_refs=("other",), source_revision="zzz").digest
    assert base.digest != sample_fact(body="different rule").digest


def test_valid_name_rejects_non_kebab():
    assert valid_name("fix-confirmed-bugs")
    assert not valid_name("Fix_Confirmed")
    assert not valid_name("trailing-")
