from agent_tools.memory_sync import codex

SAMPLE = """\
# Task Group: Hawk DeepSeek provider recovery

scope: Diagnose and restore DeepSeek creation failures on Hawk.
applies_to: cwd=/home/draven/columbus; reuse_rule=recheck live revisions before activation.

## Task 1: Diagnose and restore Hawk DeepSeek availability, success

### keywords

- DeepSeek, 61137, Machine inventory

## User preferences

- when the user said "DeepSeek 创建不了" -> start with evidence across checkout and gateway health. [Task 1]

## Reusable knowledge

- Hawk DeepSeek is not merely a Cowboy provider setting: the units probe
  loopback health first. [Task 1]

## Failures and how to do differently

- Symptom: activation rejects a candidate -> cause: origin/main advanced -> fetch first. [Task 1][Task 2]

# Task Group: Cowboy Mobile composer

scope: Fix touch-first Queue editing.

## User preferences

- when the user asked "这里应该是收起展开的 icon 吧？" -> make icons describe the action. [Task 2]
"""


def test_parse_memory_splits_groups_and_sections():
    groups = codex.parse_memory(SAMPLE)
    assert [group.title for group in groups] == [
        "Hawk DeepSeek provider recovery",
        "Cowboy Mobile composer",
    ]
    first = groups[0]
    assert first.scope.startswith("Diagnose and restore")
    assert "recheck live revisions" in first.applies_to
    sections = {bullet.section for bullet in first.bullets}
    assert sections == {codex.PREFERENCES, codex.KNOWLEDGE, codex.FAILURES}


def test_parse_memory_joins_wrapped_bullets():
    groups = codex.parse_memory(SAMPLE)
    knowledge = [b for b in groups[0].bullets if b.section == codex.KNOWLEDGE]
    assert len(knowledge) == 1
    assert "the units probe loopback health first" in knowledge[0].text


def test_bullet_statement_strips_task_provenance():
    groups = codex.parse_memory(SAMPLE)
    failure = next(b for b in groups[0].bullets if b.section == codex.FAILURES)
    assert failure.tasks == ("1", "2")
    assert "[Task" not in failure.statement
    assert failure.statement.endswith("fetch first.")


def test_keywords_section_is_not_treated_as_bullets():
    groups = codex.parse_memory(SAMPLE)
    assert all(bullet.section in codex.BULLET_SECTIONS for bullet in groups[0].bullets)


def test_render_keeps_applies_to_for_durability_judgment():
    rendered = codex.parse_memory(SAMPLE)[0].render()
    assert "Applies to:" in rendered
    assert "recheck live revisions" in rendered
    assert "User preferences:" in rendered


def test_load_missing_home_is_not_an_error(tmp_path):
    groups, revision = codex.load(tmp_path)
    assert groups == []
    assert revision == ""
