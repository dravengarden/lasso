"""Deterministic reader for the Codex memory compilation artifact.

`MEMORY.md` is compiled by Codex from rollout summaries and ad-hoc notes. It is
regenerated, so it is an input here and never an output. The structure is
regular enough to parse without a model: task groups carry a scope line, an
applies-to line, and bullet sections whose names are fixed.
"""

from __future__ import annotations

import re
import subprocess
from typing import TYPE_CHECKING

from pydantic import BaseModel, Field

if TYPE_CHECKING:
    from pathlib import Path

GROUP_PREFIX = "# Task Group: "
SECTION_PREFIX = "## "
PREFERENCES = "User preferences"
KNOWLEDGE = "Reusable knowledge"
FAILURES = "Failures and how to do differently"
BULLET_SECTIONS = (PREFERENCES, KNOWLEDGE, FAILURES)

TASK_REFERENCE = re.compile(r"\[Task ([^\]]+)\]")
SCOPE_FIELD = re.compile(r"^scope:\s*(.+)$")
APPLIES_FIELD = re.compile(r"^applies_to:\s*(.+)$")


class Bullet(BaseModel):
    """One durable statement recorded under a task group."""

    section: str
    text: str
    tasks: tuple[str, ...] = ()

    @property
    def statement(self) -> str:
        """Return the bullet without its trailing task provenance markers."""
        return TASK_REFERENCE.sub("", self.text).strip()


class TaskGroup(BaseModel):
    """A Codex task group and the durable statements it recorded."""

    title: str
    scope: str = ""
    applies_to: str = ""
    bullets: list[Bullet] = Field(default_factory=list)

    def render(self) -> str:
        """Render the group as stable text for a model request.

        The applies-to line is preserved because Codex records time-sensitivity
        there, which is the strongest available signal for durability judgment.
        """
        lines = [f"Task group: {self.title}"]
        if self.scope:
            lines.append(f"Scope: {self.scope}")
        if self.applies_to:
            lines.append(f"Applies to: {self.applies_to}")
        for section in BULLET_SECTIONS:
            selected = [bullet for bullet in self.bullets if bullet.section == section]
            if not selected:
                continue
            lines.append("")
            lines.append(f"{section}:")
            lines.extend(f"- {bullet.statement}" for bullet in selected)
        return "\n".join(lines)


def _flush(section: str, buffer: list[str], bullets: list[Bullet]) -> None:
    if not buffer:
        return
    text = " ".join(part.strip() for part in buffer).strip()
    if text:
        bullets.append(Bullet(section=section, text=text, tasks=tuple(TASK_REFERENCE.findall(text))))
    buffer.clear()


def _parse_group(block: str) -> TaskGroup:
    lines = block.splitlines()
    group = TaskGroup(title=lines[0].strip())
    section = ""
    buffer: list[str] = []
    for line in lines[1:]:
        if line.startswith(SECTION_PREFIX):
            _flush(section, buffer, group.bullets)
            section = line[len(SECTION_PREFIX) :].strip()
            continue
        if section in BULLET_SECTIONS:
            if line.startswith("- "):
                _flush(section, buffer, group.bullets)
                buffer.append(line[2:])
            elif buffer and line.strip():
                buffer.append(line)
            elif not line.strip():
                _flush(section, buffer, group.bullets)
            continue
        if section:
            continue
        if match := SCOPE_FIELD.match(line):
            group.scope = match.group(1).strip()
        elif match := APPLIES_FIELD.match(line):
            group.applies_to = match.group(1).strip()
    _flush(section, buffer, group.bullets)
    return group


def parse_memory(text: str) -> list[TaskGroup]:
    """Parse the Codex memory artifact into task groups.

    Groups are returned in file order. That order is the only supported
    freshness signal: later groups were compiled later, and no model is asked
    to reason about which statement is newer.
    """
    groups = []
    for block in text.split(f"\n{GROUP_PREFIX}"):
        stripped = block.removeprefix(GROUP_PREFIX)
        if not stripped.strip():
            continue
        groups.append(_parse_group(stripped))
    return groups


def memory_path(codex_home: Path) -> Path:
    """Return the Codex memory artifact path for a Codex home directory."""
    return codex_home / "memories" / "MEMORY.md"


def revision(codex_home: Path) -> str:
    """Return the Codex memory Git revision, or an empty string when absent.

    Codex keeps its memory directory under Git, which makes the revision a
    deterministic incremental marker; nothing here falls back to timestamps.
    """
    memories = codex_home / "memories"
    completed = subprocess.run(  # noqa: S603 - fixed argv, no shell
        ["git", "-C", str(memories), "rev-parse", "HEAD"],  # noqa: S607 - resolved from PATH by design
        check=False,
        capture_output=True,
        text=True,
    )
    return completed.stdout.strip() if completed.returncode == 0 else ""


def load(codex_home: Path) -> tuple[list[TaskGroup], str]:
    """Load task groups and the memory revision from a Codex home directory."""
    path = memory_path(codex_home)
    if not path.is_file():
        return [], ""
    return parse_memory(path.read_text(encoding="utf-8")), revision(codex_home)
