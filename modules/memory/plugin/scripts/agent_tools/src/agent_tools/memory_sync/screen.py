"""Deterministic screens for things the model was measurably bad at judging.

A first run against the real Codex corpus kept every candidate: the model
marked a fact quoting two loopback ports and a Rust symbol as durable, and
marked a rule already written into `AGENTS.md` as not covered by guidance. Both
are checkable in code, so neither stays a model decision.

Volatile candidates are dropped, because a memory that pins a port or a
revision rots silently.

Guidance coverage is not decided here. Lexical overlap was tried as a filter
and measured against a real restatement of a deployment rule: the paraphrase
shared three content words with the paragraph that mandates it and scored 0.16,
while a threshold low enough to catch it flagged half of an unrelated batch. So
overlap is used only to retrieve the guidance most worth showing the model,
which is a ranking problem it can do, rather than a judgment it cannot.
"""

from __future__ import annotations

import re

IPV4 = re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}\b")

# Ordered for reporting only; every marker is evaluated.
VOLATILE_MARKERS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("address", IPV4),
    ("port", re.compile(r":\d{4,5}\b")),
    ("revision", re.compile(r"\b[0-9a-f]{7,40}\b")),
    # Matched after IPv4 addresses are masked out, which otherwise read as versions.
    ("version", re.compile(r"\bv?\d+\.\d+(?:\.\d+)*\b")),
    ("source-symbol", re.compile(r"\b\w+\.(?:rs|go|py|ts|tsx|nix|ex|exs)::\w+")),
    ("store-path", re.compile(r"/nix/store/")),
)

WORD = re.compile(r"[a-z0-9][a-z0-9/_-]*")
STOP_WORDS = frozenset(
    {
        "a", "an", "and", "are", "as", "at", "be", "before", "both", "but", "by", "do", "does", "for",
        "from", "has", "have", "in", "is", "it", "its", "not", "of", "on", "or", "that", "the", "then",
        "this", "to", "use", "used", "when", "which", "with", "you", "your",
    }
)  # fmt: skip
MIN_TOKENS = 4
RELEVANT_STATEMENTS = 6
# A guidance rule is a paragraph or a list item. Splitting per sentence cuts a
# single rule apart and hides a restatement that spans its clauses.
STATEMENT_SPLIT = re.compile(r"\n\s*\n|\n\s*[-*+]\s|\n\s*\d+\.\s")
LEADING_MARKER = re.compile(r"^(?:[-*+]|\d+\.)\s+")
MIN_STATEMENT_TOKENS = 4


def volatile_markers(text: str) -> list[str]:
    """Return the names of volatility markers present in a candidate."""
    masked = IPV4.sub(" ", text)
    found = []
    for name, pattern in VOLATILE_MARKERS:
        subject = text if name == "address" else masked
        if pattern.search(subject):
            found.append(name)
    return found


def _tokens(text: str) -> set[str]:
    return {word for word in WORD.findall(text.lower()) if word not in STOP_WORDS and len(word) > 1}


def statements(guidance: str) -> list[str]:
    """Split guidance into its rule-sized units: paragraphs and list items."""
    found = []
    for block in STATEMENT_SPLIT.split(guidance):
        cleaned = LEADING_MARKER.sub("", " ".join(block.split()))
        if len(_tokens(cleaned)) >= MIN_STATEMENT_TOKENS:
            found.append(cleaned)
    return found


def guidance_overlap(text: str, statement: str) -> float:
    """Return the share of a candidate's meaningful words a statement also uses.

    Containment rather than similarity, so that a short rule restated inside a
    longer statement scores high instead of being diluted by its length.
    """
    candidate = _tokens(text)
    if len(candidate) < MIN_TOKENS:
        return 0.0
    reference = _tokens(statement)
    if len(reference) < MIN_STATEMENT_TOKENS:
        return 0.0
    return len(candidate & reference) / len(candidate)


def relevant_guidance(text: str, guidance: str, limit: int = RELEVANT_STATEMENTS) -> list[str]:
    """Return the guidance statements most worth checking a candidate against.

    The model reliably misses a rule buried in a long document and reliably
    spots it in a short excerpt, so retrieval carries the weight that a
    threshold could not.
    """
    scored = [(guidance_overlap(text, statement), statement) for statement in statements(guidance)]
    ranked = sorted((item for item in scored if item[0] > 0), key=lambda item: (-item[0], item[1]))
    return [statement for _, statement in ranked[:limit]]
