from agent_tools.memory_sync.screen import guidance_overlap, relevant_guidance, statements, volatile_markers

# Verbatim from a first real run: the model marked this durable despite the
# prompt naming ports as a reason to reject.
REAL_VOLATILE = (
    "Check `codex-deepseek.service` probe at `127.0.0.1:61137/healthz` and "
    "`claude-deepseek.service` probe at `127.0.0.1:61138/healthz`; "
    "`machine_cli.rs::collect_inventory` advertises adapters only when those probes succeed."
)

# Also from that run: already mandated by AGENTS.md, yet reported as uncovered.
REAL_RESTATEMENT = (
    "Before rebuilding or activating a Hawk candidate, fetch and integrate the latest origin/main; "
    "do not activate against a stale base."
)
REAL_GUIDANCE = (
    "For Hawk NixOS, deployment authority is a clean Git commit, not a working tree and not a pushed "
    "branch: fetch first, integrate both current origin/main and the active deployment revision, "
    "commit, build the exact commit, and use the host-owned transactional activator."
)

DURABLE = (
    "Preserve UIKit ownership of touch, selection, and paste in the Mobile composer. "
    "Do not add custom touch interception or delayed refocus on iOS."
)


def test_detects_the_markers_the_model_let_through():
    assert set(volatile_markers(REAL_VOLATILE)) == {"address", "port", "source-symbol"}


def test_detects_each_marker_independently():
    assert volatile_markers("pinned at revision 5dc40bfd0246eeb") == ["revision"]
    assert volatile_markers("requires Cowboy 1.2.3 or newer") == ["version"]
    assert volatile_markers("reads /nix/store/abc-foo/bin/x") == ["store-path"]
    assert volatile_markers("listens on 127.0.0.1:61137") == ["address", "port"]


def test_an_ip_address_is_not_reported_as_a_version():
    assert "version" not in volatile_markers("bind 192.168.0.96 for the LAN")


def test_a_durable_rule_has_no_markers():
    assert volatile_markers(DURABLE) == []


def test_ordinary_prose_numbers_are_not_ports():
    assert volatile_markers("Review the 2026 migration plan in two steps.") == []


STATEMENTS = (
    "Preserve unrelated working-tree changes and prefer narrow edits.",
    REAL_GUIDANCE,
    "Mobile stays touch-first and progressively disclosed.",
    "Never commit secrets or credentials to the repository.",
    "- Run the owning deterministic gate before asking for review.",
)
DOCUMENT = "\n\n".join(STATEMENTS)


def test_statements_split_paragraphs_and_list_items():
    found = statements(DOCUMENT)
    assert len(found) == 5
    assert found[1] == REAL_GUIDANCE
    assert found[4].startswith("Run the owning")


def test_statements_drop_fragments_too_short_to_rank():
    assert statements("# Title\n\nok\n\nA real rule with several meaningful words here.") == [
        "A real rule with several meaningful words here."
    ]


def test_relevant_guidance_ranks_the_mandating_statement_first():
    ranked = relevant_guidance(REAL_RESTATEMENT, DOCUMENT)
    assert ranked[0] == REAL_GUIDANCE


def test_relevant_guidance_is_bounded_and_skips_unrelated_statements():
    ranked = relevant_guidance(DURABLE, DOCUMENT, limit=2)
    assert len(ranked) <= 2
    assert REAL_GUIDANCE not in ranked


def test_relevant_guidance_is_empty_without_guidance():
    assert relevant_guidance(REAL_RESTATEMENT, "") == []


def test_overlap_is_containment_not_similarity():
    padded = REAL_GUIDANCE + " " + ("extra clause about unrelated topics " * 20)
    assert guidance_overlap(REAL_RESTATEMENT, padded) == guidance_overlap(REAL_RESTATEMENT, REAL_GUIDANCE)


def test_trailing_punctuation_does_not_split_a_token():
    assert guidance_overlap("stale base.", "a stale base rule") == guidance_overlap("stale base", "a stale base rule")


def test_empty_inputs_score_zero():
    assert guidance_overlap("", REAL_GUIDANCE) == 0.0
    assert guidance_overlap(REAL_RESTATEMENT, "") == 0.0
    assert guidance_overlap("two words", REAL_GUIDANCE) == 0.0
