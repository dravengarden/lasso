"""Portable fact files used as reviewed extraction staging.

Claude Code uses one Markdown file per fact plus a `MEMORY.md` index and
tolerates frontmatter keys it does not know. This module renders that portable
shape in a staging directory; the plugin does not write Claude's live store.
"""

from __future__ import annotations

import hashlib
import re
from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field

FactType = Literal["user", "feedback", "project", "reference"]
FactScope = Literal["project", "global"]
# Source identifies the originating store shape. ``shared`` means the portable
# follower-runtime shape, not a shared live directory or a plugin write target.
FactSource = Literal["codex", "shared"]

INDEX_NAME = "MEMORY.md"
INDEX_LINE = re.compile(r"^\s*-\s\[.+?\]\(([^)]+)\.md\)\s*—\s.*$")
NAME_PATTERN = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")
FRONTMATTER_FENCE = "---"
QUOTE_PAIR = 2


class Fact(BaseModel):
    """One durable fact, addressable by a stable kebab-case name."""

    name: str
    title: str
    description: str
    type: FactType
    scope: FactScope
    body: str
    why: str = ""
    how_to_apply: str = ""
    source: FactSource
    source_home: str = ""
    source_refs: tuple[str, ...] = ()
    source_revision: str = ""
    superseded_by: str = ""
    stale_paths: tuple[str, ...] = ()

    @property
    def digest(self) -> str:
        """Return a content digest used for idempotent rewrites."""
        fields = (self.title, self.description, self.body, self.why, self.how_to_apply)
        return hashlib.sha256("\0".join(fields).encode("utf-8")).hexdigest()[:16]

    def render(self) -> str:
        """Render the fact as a memory file both runtimes can read."""
        lines = [FRONTMATTER_FENCE, f"name: {self.name}", f"description: {_scalar(self.description)}", "metadata:"]
        lines.append(f"  type: {self.type}")
        lines.append(f"  scope: {self.scope}")
        lines.append(f"  title: {_scalar(self.title)}")
        lines.append(f"  source: {self.source}")
        if self.source_home:
            lines.append(f"  source_home: {_scalar(self.source_home)}")
        if self.source_refs:
            lines.append(f"  source_refs: {_scalar(' | '.join(self.source_refs))}")
        if self.source_revision:
            lines.append(f"  source_revision: {self.source_revision}")
        if self.superseded_by:
            lines.append(f"  superseded_by: {self.superseded_by}")
        if self.stale_paths:
            lines.append(f"  stale_paths: {_scalar(' | '.join(self.stale_paths))}")
        lines.append(f"  digest: {self.digest}")
        lines.append(FRONTMATTER_FENCE)
        lines.append("")
        lines.append(self.body.strip())
        if self.why:
            lines.extend(["", f"**Why:** {self.why.strip()}"])
        if self.how_to_apply:
            lines.extend(["", f"**How to apply:** {self.how_to_apply.strip()}"])
        return "\n".join(lines) + "\n"

    def index_line(self) -> str:
        """Render the `MEMORY.md` index line both stores parse."""
        return f"- [{self.title}]({self.name}.md) — [{self.scope}/{self.type}] {self.description}"


def _scalar(value: str) -> str:
    """Quote a frontmatter scalar so punctuation cannot break the document."""
    escaped = value.replace("\\", "\\\\").replace('"', '\\"').replace("\n", " ")
    return f'"{escaped}"'


def _unquote(value: str) -> str:
    stripped = value.strip()
    if len(stripped) >= QUOTE_PAIR and stripped[0] == stripped[-1] and stripped[0] in {'"', "'"}:
        inner = stripped[1:-1]
        return inner.replace('\\"', '"').replace("\\\\", "\\")
    return stripped


class RawFact(BaseModel):
    """A fact file read from any store, without assuming this plugin wrote it."""

    path: Path
    name: str
    description: str = ""
    metadata: dict[str, str] = Field(default_factory=dict)
    body: str = ""

    @property
    def source(self) -> str:
        """Return the recorded origin runtime, or an empty string when unknown."""
        return self.metadata.get("source", "")

    @property
    def source_home(self) -> str:
        """Return the store this fact was read out of, or an empty string."""
        return self.metadata.get("source_home", "")


def canonical_home(path: Path | str) -> str:
    """Return a comparable form of a memory store path.

    `~/.codex`, `/home/u/.codex`, and `/home/u/.codex/` all name one store, so
    provenance is compared on the resolved path rather than the literal string.
    """
    return str(Path(path).expanduser().resolve())


def read_fact(path: Path) -> RawFact:
    """Read one fact file tolerantly.

    Files written by Claude Code carry a subset or a superset of these keys.
    Missing frontmatter is not an error: the file stem is the name, which
    matches how the store recovers from hand-authored files.
    """
    text = path.read_text(encoding="utf-8")
    fact = RawFact(path=path, name=path.stem)
    lines = text.splitlines()
    if not lines or lines[0].strip() != FRONTMATTER_FENCE:
        fact.body = text.strip()
        return fact
    section = ""
    for index, line in enumerate(lines[1:], start=2):
        if line.strip() == FRONTMATTER_FENCE:
            fact.body = "\n".join(lines[index:]).strip()
            break
        if not line.strip():
            continue
        if not line.startswith(" ") and line.rstrip().endswith(":") and ":" in line:
            section = line.split(":", 1)[0].strip()
            continue
        key, separator, value = line.strip().partition(":")
        if not separator:
            continue
        key = key.strip()
        parsed = _unquote(value)
        if line.startswith(" ") and section:
            fact.metadata[key] = parsed
        elif key == "name":
            fact.name = parsed or fact.name
        elif key == "description":
            fact.description = parsed
    return fact


def read_facts(directory: Path) -> list[RawFact]:
    """Read every fact file in a directory, skipping the index and archive."""
    if not directory.is_dir():
        return []
    return [
        read_fact(path)
        for path in sorted(directory.glob("*.md"))
        if path.name != INDEX_NAME and not path.name.startswith(".")
    ]


def write_facts(directory: Path, facts: list[Fact]) -> list[Path]:
    """Write fact files and refresh the index, preserving hand-authored lines.

    Managed index lines are replaced; anything else in `MEMORY.md` survives,
    which is the same contract both stores honor.
    """
    directory.mkdir(parents=True, exist_ok=True)
    written = []
    for fact in facts:
        path = directory / f"{fact.name}.md"
        path.write_text(fact.render(), encoding="utf-8")
        written.append(path)
    _write_index(directory, facts)
    return written


def _write_index(directory: Path, facts: list[Fact]) -> None:
    index = directory / INDEX_NAME
    managed = {fact.name: fact.index_line() for fact in facts}
    kept: list[str] = []
    if index.is_file():
        for line in index.read_text(encoding="utf-8").splitlines():
            match = INDEX_LINE.match(line)
            if match and match.group(1) in managed:
                continue
            kept.append(line)
    while kept and not kept[-1].strip():
        kept.pop()
    body = [*kept, *[managed[name] for name in sorted(managed)]]
    index.write_text("\n".join(body).strip() + "\n", encoding="utf-8")


def valid_name(name: str) -> bool:
    """Return whether a fact name is a usable kebab-case file stem."""
    return bool(NAME_PATTERN.match(name))
