"""Deterministic staleness detection across memory stores.

This runs without a model and without a network. A fact that names a path which
no longer exists is the cheapest reliable staleness signal available, and it is
the one signal a model is measurably bad at: it cannot check the filesystem.

Nothing here deletes. Current practice in temporal memory systems is to
invalidate a superseded fact while keeping it auditable.
"""

from __future__ import annotations

import re
from pathlib import Path

from pydantic import BaseModel, Field

from agent_tools.memory_sync.facts import RawFact, read_fact, read_facts

BACKTICKED = re.compile(r"`([^`\n]+)`")
# The lookbehind matters: without it the tail of a relative path such as
# `web/src/mdlive/PITFALLS.md` reads as the absolute `/src/mdlive/PITFALLS.md`
# and is reported missing, which is exactly the false staleness to avoid.
ABSOLUTE_PATH = re.compile(r"(?<![\w.\-])(/(?:[\w.\-]+/)+[\w.\-]+)")
# Pseudo-filesystems and the live-system tree are generated per boot, so their
# absence says nothing about a fact. An ordinary path that merely happens to be
# short-lived still counts: a reference to a directory that is gone is stale.
IGNORED_PREFIXES = ("/proc/", "/sys/", "/dev/", "/run/")


class Finding(BaseModel):
    """One fact that references paths which no longer exist."""

    name: str
    path: Path
    missing: list[str] = Field(default_factory=list)
    source: str = ""


def referenced_paths(text: str) -> list[str]:
    """Return absolute filesystem paths a fact refers to.

    Only absolute paths are considered. A relative token cannot be resolved
    without knowing which checkout the fact was about, and guessing would
    produce false staleness.
    """
    candidates: list[str] = []
    for value in BACKTICKED.findall(text):
        candidates.extend(ABSOLUTE_PATH.findall(value))
    candidates.extend(ABSOLUTE_PATH.findall(BACKTICKED.sub(" ", text)))
    seen: dict[str, None] = {}
    for candidate in candidates:
        cleaned = candidate.rstrip(".,;:)")
        if cleaned.startswith(IGNORED_PREFIXES):
            continue
        seen.setdefault(cleaned, None)
    return list(seen)


def inspect(fact: RawFact) -> Finding | None:
    """Return a finding when a fact references a path that is gone."""
    text = f"{fact.description}\n{fact.body}"
    missing = [path for path in referenced_paths(text) if not Path(path).exists()]
    if not missing:
        return None
    return Finding(name=fact.name, path=fact.path, missing=missing, source=fact.source)


def scan(locations: list[Path]) -> list[Finding]:
    """Scan every fact directory or Markdown file and report stale references."""
    findings = []
    seen: set[Path] = set()
    for location in locations:
        resolved = location.expanduser().resolve()
        if resolved in seen:
            continue
        seen.add(resolved)
        facts = [read_fact(location)] if location.is_file() else read_facts(location)
        findings.extend(finding for fact in facts if (finding := inspect(fact)))
    return findings


def claude_memory_dirs(claude_home: Path) -> list[Path]:
    """Return every project-scoped Claude Code memory directory."""
    projects = claude_home / "projects"
    if not projects.is_dir():
        return []
    return sorted(path for path in projects.glob("*/memory") if path.is_dir())


def grok_memory_locations(grok_home: Path) -> list[Path]:
    """Return Grok's runtime-owned global, workspace, and session memory locations."""
    memory = grok_home / "memory"
    if not memory.is_dir():
        return []
    locations: list[Path] = []
    global_memory = memory / "MEMORY.md"
    if global_memory.is_file():
        locations.append(global_memory)
    for workspace in sorted(path for path in memory.iterdir() if path.is_dir()):
        workspace_memory = workspace / "MEMORY.md"
        if workspace_memory.is_file():
            locations.append(workspace_memory)
        sessions = workspace / "sessions"
        if sessions.is_dir():
            locations.append(sessions)
    return locations
