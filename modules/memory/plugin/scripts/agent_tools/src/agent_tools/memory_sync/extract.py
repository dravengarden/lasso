"""Turn Codex task groups into reviewable fact files.

The split is deliberate and follows the measured failure modes of asking a
model to track freshness: the model only proposes candidates and judges whether
a statement is durable, in scope, and already covered by repository guidance.
Ordering, supersession, and deduplication stay in code, keyed on the position
of a task group in the Codex artifact.

Contradiction detection runs before deduplication. A synchronous dedup gate
placed first silently discards the conflicting write before anything can
observe the conflict.
"""

from __future__ import annotations

import json
import re
from typing import TYPE_CHECKING

from pydantic import BaseModel, Field, ValidationError

if TYPE_CHECKING:
    from pathlib import Path

from agent_tools.memory_sync import codex, gateway, screen
from agent_tools.memory_sync.facts import Fact, FactScope, FactType, canonical_home, valid_name

JSON_BLOCK = re.compile(r"\{.*\}", re.DOTALL)

INSTRUCTIONS = """\
You convert an agent's recorded task memory into durable, reusable facts.

Return ONLY a JSON object of this shape, with no prose and no code fence:

{"facts": [{"name": "kebab-case-stem", "title": "Short title",
  "description": "One sentence, under 160 characters",
  "type": "user|feedback|project|reference", "scope": "project|global",
  "body": "Imperative statement of the rule or fact",
  "why": "Why it holds", "how_to_apply": "What to do differently",
  "durable": true, "covered_by_guidance": false, "reason": "one short clause"}]}

Type meanings:
- user: who the person is, their role, expertise, standing preferences.
- feedback: guidance on how an agent should work; corrections and confirmed approaches.
- project: ongoing work, goals, or constraints not derivable from code or history.
- reference: a pointer to an external resource.

Rules:
1. Emit at most one fact per input bullet. Skip a bullet entirely rather than
   inventing content for it.
2. Set durable=false when the statement is tied to one incident, a specific
   revision, a live service state, a port, a version, or anything the input
   marks as needing a recheck. Prefer durable=false when uncertain.
3. Set covered_by_guidance=true when the repository guidance below already
   states the rule. Do not restate guidance as a memory.
4. Choose `name` from the meaning, not from the wording. Two bullets that
   express the same underlying rule must produce the same name so they can be
   merged later. Names are lowercase kebab-case.
5. Never decide which of two statements is newer or which supersedes another.
   Never mention ordering, recency, or supersession.
6. Write body, why, and how_to_apply in English, imperative, self-contained.
   A reader must not need the original task context.
"""


class Candidate(BaseModel):
    """One fact proposed by the model, before deterministic reconciliation."""

    name: str
    title: str
    description: str
    type: FactType
    scope: FactScope
    body: str
    why: str = ""
    how_to_apply: str = ""
    durable: bool = False
    covered_by_guidance: bool = False
    reason: str = ""


class GroupResult(BaseModel):
    """The model's answer for one task group."""

    facts: list[Candidate] = Field(default_factory=list)


class Conflict(BaseModel):
    """Two candidates that share a name but disagree on content."""

    name: str
    kept_from: str
    dropped_from: str


class Report(BaseModel):
    """Outcome of one extraction run."""

    revision: str = ""
    groups: int = 0
    bullets: int = 0
    proposed: int = 0
    dropped_not_durable: int = 0
    dropped_covered: int = 0
    dropped_volatile: int = 0
    dropped_invalid: int = 0
    merged: int = 0
    conflicts: list[Conflict] = Field(default_factory=list)
    facts: list[Fact] = Field(default_factory=list)
    usage: gateway.Usage = Field(default_factory=gateway.Usage)
    failures: list[str] = Field(default_factory=list)


def build_system_prompt(guidance: str) -> str:
    """Build the byte-stable prefix shared by every call in a batch.

    Nothing that changes between calls belongs here. The already-extracted set
    in particular must stay out, or the prefix cache misses on every request.
    """
    sections = [INSTRUCTIONS]
    if guidance.strip():
        sections.append("Repository guidance already in force (do not restate it as a memory):\n\n" + guidance.strip())
    return "\n\n".join(sections)


def build_user_prompt(group: codex.TaskGroup, guidance: str) -> str:
    """Build the per-call message, with the guidance most relevant to this group.

    The full guidance is already in the cached prefix, but a rule buried in a
    long document is routinely missed. Repeating the closest statements here,
    where they are short and adjacent to the input, is what makes rule 3
    enforceable.
    """
    rendered = group.render()
    excerpts = screen.relevant_guidance(rendered, guidance) if guidance.strip() else []
    if not excerpts:
        return rendered
    lines = [rendered, "", "Guidance statements closest to this group. A bullet that any of these already"]
    lines.append("mandates must be returned with covered_by_guidance=true:")
    lines.extend(f"- {excerpt}" for excerpt in excerpts)
    return "\n".join(lines)


def parse_result(text: str) -> GroupResult:
    """Parse a model answer, tolerating a code fence or surrounding prose."""
    match = JSON_BLOCK.search(text)
    if not match:
        message = "model answer contained no JSON object"
        raise ValueError(message)
    try:
        payload = json.loads(match.group(0))
    except json.JSONDecodeError as error:
        message = f"model answer was not valid JSON: {error}"
        raise ValueError(message) from error
    try:
        return GroupResult.model_validate(payload)
    except ValidationError as error:
        message = f"model answer did not match the fact schema: {error}"
        raise ValueError(message) from error


def _to_fact(candidate: Candidate, group: codex.TaskGroup, revision: str, source_home: str) -> Fact:
    return Fact(
        name=candidate.name,
        title=candidate.title.strip() or candidate.name.replace("-", " ").capitalize(),
        description=candidate.description.strip(),
        type=candidate.type,
        scope=candidate.scope,
        body=candidate.body.strip(),
        why=candidate.why.strip(),
        how_to_apply=candidate.how_to_apply.strip(),
        source="codex",
        # Recorded so the reverse direction can tell a sync between two Codex
        # homes apart from a write back into the store this came from.
        source_home=source_home,
        source_refs=(group.title,),
        source_revision=revision,
    )


def reconcile(proposed: list[tuple[int, Fact]], report: Report) -> list[Fact]:
    """Merge same-named facts deterministically, recording conflicts first.

    `proposed` carries the source group index. A later group wins on content,
    because file order is the only freshness signal the artifact provides. The
    losing text is not discarded silently: it is recorded as a conflict, and
    both source references are preserved on the surviving fact.
    """
    by_name: dict[str, tuple[int, Fact]] = {}
    for index, fact in proposed:
        existing = by_name.get(fact.name)
        if existing is None:
            by_name[fact.name] = (index, fact)
            continue
        previous_index, previous = existing
        report.merged += 1
        if previous.digest != fact.digest:
            report.conflicts.append(
                Conflict(
                    name=fact.name,
                    kept_from=fact.source_refs[-1] if index >= previous_index else previous.source_refs[-1],
                    dropped_from=previous.source_refs[-1] if index >= previous_index else fact.source_refs[-1],
                )
            )
        winner, loser = (fact, previous) if index >= previous_index else (previous, fact)
        refs = tuple(dict.fromkeys([*loser.source_refs, *winner.source_refs]))
        by_name[fact.name] = (max(index, previous_index), winner.model_copy(update={"source_refs": refs}))
    return [fact for _, fact in sorted(by_name.values(), key=lambda item: item[1].name)]


def extract(  # noqa: PLR0913 - every argument is an independent run input
    groups: list[codex.TaskGroup],
    revision: str,
    guidance: str,
    *,
    source_home: Path,
    endpoint: str,
    model: str,
    limit: int = 0,
    max_tokens: int = gateway.DEFAULT_MAX_TOKENS,
    keep_volatile: bool = False,
) -> Report:
    """Run the extraction pipeline over Codex task groups."""
    origin = canonical_home(source_home)
    report = Report(revision=revision, groups=len(groups))
    system = build_system_prompt(guidance)
    selected = groups[:limit] if limit else groups
    report.groups = len(selected)
    proposed: list[tuple[int, Fact]] = []
    for index, group in enumerate(selected):
        report.bullets += len(group.bullets)
        try:
            completion = gateway.complete(
                system,
                build_user_prompt(group, guidance),
                endpoint=endpoint,
                model=model,
                max_tokens=max_tokens,
            )
        except gateway.GatewayError as error:
            report.failures.append(f"{group.title}: {error}")
            continue
        report.usage = report.usage.merge(completion.usage)
        try:
            result = parse_result(completion.text)
        except ValueError as error:
            report.failures.append(f"{group.title}: {error}")
            continue
        for candidate in result.facts:
            report.proposed += 1
            if not candidate.durable:
                report.dropped_not_durable += 1
                continue
            if candidate.covered_by_guidance:
                report.dropped_covered += 1
                continue
            if not valid_name(candidate.name) or not candidate.body.strip() or not candidate.description.strip():
                report.dropped_invalid += 1
                continue
            text = f"{candidate.description}\n{candidate.body}\n{candidate.how_to_apply}"
            if not keep_volatile and screen.volatile_markers(text):
                report.dropped_volatile += 1
                continue
            proposed.append((index, _to_fact(candidate, group, revision, origin)))
    report.facts = reconcile(proposed, report)
    return report


def read_guidance(paths: list[Path]) -> str:
    """Concatenate repository guidance used to suppress already-stated rules."""
    parts = [path.read_text(encoding="utf-8").strip() for path in paths if path.is_file()]
    return "\n\n".join(part for part in parts if part)
