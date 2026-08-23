from pathlib import Path

import pytest

from agent_tools.memory_sync import codex, extract
from agent_tools.memory_sync.extract import Candidate, Report, build_system_prompt, parse_result, reconcile
from agent_tools.memory_sync.facts import Fact

SOURCE_HOME = Path("/stores/under-test")


def fact(name: str, group: str, body: str = "rule") -> Fact:
    return Fact(
        name=name,
        title=name,
        description="d",
        type="feedback",
        scope="global",
        body=body,
        source="codex",
        source_refs=(group,),
    )


def test_parse_result_accepts_a_fenced_answer():
    answer = '```json\n{"facts": [{"name": "a-b", "title": "T", "description": "D", "type": "project",\n'
    answer += '"scope": "global", "body": "B", "durable": true}]}\n```'
    result = parse_result(answer)
    assert len(result.facts) == 1
    assert result.facts[0].name == "a-b"


def test_parse_result_rejects_a_non_json_answer():
    with pytest.raises(ValueError, match="no JSON object"):
        parse_result("I cannot help with that.")


def test_parse_result_rejects_an_unknown_type():
    answer = '{"facts": [{"name": "a", "title": "t", "description": "d",'
    answer += ' "type": "lore", "scope": "global", "body": "b"}]}'
    with pytest.raises(ValueError, match="fact schema"):
        parse_result(answer)


def test_candidate_defaults_to_not_durable():
    candidate = Candidate(name="a", title="t", description="d", type="project", scope="global", body="b")
    assert candidate.durable is False


def test_system_prompt_is_stable_and_excludes_per_call_state():
    first = build_system_prompt("guidance text")
    second = build_system_prompt("guidance text")
    assert first == second
    assert "guidance text" in first
    assert "never decide which of two statements is newer" in first.lower()


def test_reconcile_keeps_the_later_group_and_records_the_conflict():
    report = Report()
    kept = reconcile([(0, fact("r", "early", "old rule")), (1, fact("r", "late", "new rule"))], report)
    assert [f.body for f in kept] == ["new rule"]
    assert report.merged == 1
    assert [(c.kept_from, c.dropped_from) for c in report.conflicts] == [("late", "early")]


def test_reconcile_is_order_independent_for_the_winner():
    report = Report()
    kept = reconcile([(1, fact("r", "late", "new rule")), (0, fact("r", "early", "old rule"))], report)
    assert [f.body for f in kept] == ["new rule"]


def test_reconcile_preserves_every_source_reference():
    report = Report()
    kept = reconcile([(0, fact("r", "early", "same")), (1, fact("r", "late", "same"))], report)
    assert kept[0].source_refs == ("early", "late")


def test_identical_duplicates_merge_without_a_conflict():
    report = Report()
    reconcile([(0, fact("r", "early", "same")), (1, fact("r", "late", "same"))], report)
    assert report.merged == 1
    assert report.conflicts == []


def test_extract_drops_non_durable_and_covered_candidates(monkeypatch):
    answer = """{"facts": [
      {"name": "keep-me", "title": "K", "description": "D", "type": "feedback",
       "scope": "global", "body": "B", "durable": true},
      {"name": "too-specific", "title": "T", "description": "D", "type": "project",
       "scope": "project", "body": "B", "durable": false},
      {"name": "already-known", "title": "A", "description": "D", "type": "project",
       "scope": "global", "body": "B", "durable": true, "covered_by_guidance": true},
      {"name": "Bad Name", "title": "B", "description": "D", "type": "project",
       "scope": "global", "body": "B", "durable": true}
    ]}"""
    monkeypatch.setattr(
        extract.gateway,
        "complete",
        lambda *_args, **_kwargs: extract.gateway.Completion(text=answer, usage=extract.gateway.Usage()),
    )
    groups = [codex.TaskGroup(title="G", bullets=[])]
    report = extract.extract(
        groups, "rev", "", source_home=SOURCE_HOME, endpoint="http://127.0.0.1:1/v1/messages", model="m"
    )
    assert report.proposed == 4
    assert report.dropped_not_durable == 1
    assert report.dropped_covered == 1
    assert report.dropped_invalid == 1
    assert [f.name for f in report.facts] == ["keep-me"]
    assert report.facts[0].source == "codex"
    assert report.facts[0].source_revision == "rev"
    # Recorded so the reverse direction can tell two Codex stores apart.
    assert report.facts[0].source_home == str(SOURCE_HOME)


def test_extract_drops_volatile_candidates_the_model_kept(monkeypatch):
    answer = """{"facts": [
      {"name": "health-probes", "title": "H", "description": "D", "type": "reference",
       "scope": "project", "body": "Probe 127.0.0.1:61137/healthz before acting.", "durable": true}
    ]}"""
    monkeypatch.setattr(
        extract.gateway,
        "complete",
        lambda *_args, **_kwargs: extract.gateway.Completion(text=answer, usage=extract.gateway.Usage()),
    )
    groups = [codex.TaskGroup(title="G")]
    report = extract.extract(
        groups, "rev", "", source_home=SOURCE_HOME, endpoint="http://127.0.0.1:1/v1/messages", model="m"
    )
    assert report.dropped_volatile == 1
    assert report.facts == []

    kept = extract.extract(
        groups,
        "rev",
        "",
        source_home=SOURCE_HOME,
        endpoint="http://127.0.0.1:1/v1/messages",
        model="m",
        keep_volatile=True,
    )
    assert kept.dropped_volatile == 0
    assert [f.name for f in kept.facts] == ["health-probes"]


def test_user_prompt_carries_the_guidance_closest_to_the_group():
    group = codex.TaskGroup(
        title="Hawk activation",
        bullets=[
            codex.Bullet(
                section=codex.FAILURES,
                text="Symptom: activation rejects a candidate -> cause: origin/main advanced -> fetch first.",
            )
        ],
    )
    unrelated = "Mobile stays touch-first and progressively disclosed."
    mandating = (
        "Deployment authority is a clean Git commit: fetch first, integrate current origin/main, "
        "then build the exact commit."
    )
    guidance = f"{unrelated}\n\n{mandating}"
    prompt = extract.build_user_prompt(group, guidance)
    assert "covered_by_guidance=true" in prompt
    assert "fetch first, integrate current origin/main" in prompt
    assert "touch-first" not in prompt


def test_user_prompt_is_just_the_group_without_guidance():
    group = codex.TaskGroup(title="G")
    assert extract.build_user_prompt(group, "") == group.render()


def test_extract_records_a_gateway_failure_without_losing_other_groups(monkeypatch):
    answer = '{"facts": [{"name": "ok", "title": "T", "description": "D",'
    answer += ' "type": "project", "scope": "global", "body": "B", "durable": true}]}'
    calls = {"n": 0}

    def fake(*_args, **_kwargs):
        calls["n"] += 1
        if calls["n"] == 1:
            raise extract.gateway.GatewayError("connection refused")
        return extract.gateway.Completion(text=answer, usage=extract.gateway.Usage())

    monkeypatch.setattr(extract.gateway, "complete", fake)
    groups = [codex.TaskGroup(title="A"), codex.TaskGroup(title="B")]
    report = extract.extract(
        groups, "rev", "", source_home=SOURCE_HOME, endpoint="http://127.0.0.1:1/v1/messages", model="m"
    )
    assert len(report.failures) == 1
    assert [f.name for f in report.facts] == ["ok"]


def test_extract_limit_restricts_the_batch(monkeypatch):
    seen = []

    def fake(_system, user, **_kwargs):
        seen.append(user)
        return extract.gateway.Completion(text='{"facts": []}', usage=extract.gateway.Usage())

    monkeypatch.setattr(extract.gateway, "complete", fake)
    groups = [codex.TaskGroup(title=f"G{index}") for index in range(5)]
    report = extract.extract(
        groups, "", "", source_home=SOURCE_HOME, endpoint="http://127.0.0.1:1/v1/messages", model="m", limit=2
    )
    assert len(seen) == 2
    assert report.groups == 2


def test_extract_sends_a_byte_identical_prefix(monkeypatch):
    prefixes = []

    def fake(system, _user, **_kwargs):
        prefixes.append(system)
        return extract.gateway.Completion(text='{"facts": []}', usage=extract.gateway.Usage())

    monkeypatch.setattr(extract.gateway, "complete", fake)
    groups = [codex.TaskGroup(title=f"G{index}") for index in range(3)]
    extract.extract(
        groups, "rev", "guidance", source_home=SOURCE_HOME, endpoint="http://127.0.0.1:1/v1/messages", model="m"
    )
    assert len(set(prefixes)) == 1
