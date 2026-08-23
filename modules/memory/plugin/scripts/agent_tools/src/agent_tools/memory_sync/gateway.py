"""Minimal DeepSeek client that borrows an existing loopback gateway.

This plugin never holds a DeepSeek credential. It posts to a host model
gateway on loopback, which replaces client authorization with its own private
credential and reports content-free usage to Cowboy Machine. The default is the
Anthropic Messages gateway because its wire contract is stable and DeepSeek's
prefix cache needs no explicit cache markers.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

from pydantic import BaseModel

DEFAULT_ENDPOINT = "http://127.0.0.1:61138/v1/messages"
DEFAULT_MODEL = "deepseek-v4-flash"
# Thinking is disabled by default, so this budget only has to hold the JSON
# answer. With thinking on, DeepSeek bills it against the same budget and a
# structured answer is routinely truncated before its closing brace.
DEFAULT_MAX_TOKENS = 8192
DEFAULT_TIMEOUT = 120.0
LOOPBACK_HOSTS = frozenset({"127.0.0.1", "localhost", "::1"})


class GatewayError(RuntimeError):
    """Raised when the gateway is unreachable or answers unusably."""


class Usage(BaseModel):
    """Token accounting for one completion.

    Cache fields stay `None` when the provider did not report them, so an
    absent observation is never rendered as a zero cache hit.
    """

    input_tokens: int = 0
    output_tokens: int = 0
    cache_read_tokens: int | None = None
    cache_write_tokens: int | None = None

    def merge(self, other: Usage) -> Usage:
        """Return the element-wise sum, preserving absent cache observations."""
        return Usage(
            input_tokens=self.input_tokens + other.input_tokens,
            output_tokens=self.output_tokens + other.output_tokens,
            cache_read_tokens=_add(self.cache_read_tokens, other.cache_read_tokens),
            cache_write_tokens=_add(self.cache_write_tokens, other.cache_write_tokens),
        )


def _add(left: int | None, right: int | None) -> int | None:
    if left is None and right is None:
        return None
    return (left or 0) + (right or 0)


class Completion(BaseModel):
    """One model answer plus its usage."""

    text: str
    usage: Usage


def _require_loopback(endpoint: str) -> None:
    parsed = urllib.parse.urlparse(endpoint)
    if parsed.scheme not in {"http", "https"}:
        message = f"gateway endpoint must be http(s): {endpoint}"
        raise GatewayError(message)
    if parsed.hostname not in LOOPBACK_HOSTS:
        message = f"gateway endpoint must stay on loopback so no credential leaves the host: {endpoint}"
        raise GatewayError(message)


def _extract_text(payload: dict[str, Any]) -> str:
    blocks = payload.get("content")
    if not isinstance(blocks, list):
        message = "gateway response contained no content blocks"
        raise GatewayError(message)
    reason = payload.get("stop_reason", "unknown")
    if reason == "max_tokens":
        # A truncated answer can never parse as strict JSON, and reporting the
        # budget is far more actionable than the decoder's column number.
        message = "gateway answer hit the output budget and was truncated; raise --max-tokens"
        raise GatewayError(message)
    parts = [block.get("text", "") for block in blocks if isinstance(block, dict) and block.get("type") == "text"]
    text = "".join(parts).strip()
    if not text:
        kinds = sorted({block.get("type", "?") for block in blocks if isinstance(block, dict)})
        message = f"gateway returned no text (stop_reason={reason}, blocks={kinds or ['none']})"
        raise GatewayError(message)
    return text


def _extract_usage(payload: dict[str, Any]) -> Usage:
    raw = payload.get("usage")
    if not isinstance(raw, dict):
        return Usage()
    return Usage(
        input_tokens=int(raw.get("input_tokens") or 0),
        output_tokens=int(raw.get("output_tokens") or 0),
        cache_read_tokens=_optional_int(raw.get("cache_read_input_tokens")),
        cache_write_tokens=_optional_int(raw.get("cache_creation_input_tokens")),
    )


def _optional_int(value: object) -> int | None:
    return int(value) if isinstance(value, (int, float)) else None


def complete(  # noqa: PLR0913 - request knobs stay explicit at the call site
    system: str,
    user: str,
    *,
    endpoint: str = DEFAULT_ENDPOINT,
    model: str = DEFAULT_MODEL,
    max_tokens: int = DEFAULT_MAX_TOKENS,
    timeout: float = DEFAULT_TIMEOUT,
    thinking: bool = False,
) -> Completion:
    """Send one non-streaming completion through the loopback gateway.

    `system` must be byte-identical across calls in a batch. DeepSeek's cache
    matches on a full prefix unit, so any per-call state belongs in `user`.

    Thinking is off by default. This task asks for a schema-constrained answer
    the model does not have to reason its way toward, and leaving it on tripled
    output cost while truncating the answer.
    """
    _require_loopback(endpoint)
    request_body: dict[str, Any] = {
        "model": model,
        "max_tokens": max_tokens,
        # Extraction should be reproducible: two runs over an unchanged corpus
        # must not disagree about what is durable.
        "temperature": 0,
        "system": [{"type": "text", "text": system}],
        "messages": [{"role": "user", "content": [{"type": "text", "text": user}]}],
    }
    if not thinking:
        request_body["thinking"] = {"type": "disabled"}
    body = json.dumps(request_body).encode("utf-8")
    request = urllib.request.Request(  # noqa: S310 - scheme and host checked above
        endpoint,
        data=body,
        headers={"content-type": "application/json", "anthropic-version": "2023-06-01"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:  # noqa: S310 - loopback only
            payload = json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as error:  # pragma: no cover - depends on a live gateway
        detail = error.read().decode("utf-8", errors="replace")[:500]
        message = f"gateway returned HTTP {error.code}: {detail}"
        raise GatewayError(message) from error
    except (urllib.error.URLError, TimeoutError) as error:  # pragma: no cover - depends on a live gateway
        message = f"gateway at {endpoint} is unreachable: {error}"
        raise GatewayError(message) from error
    except json.JSONDecodeError as error:  # pragma: no cover - depends on a live gateway
        message = "gateway returned a non-JSON body"
        raise GatewayError(message) from error
    if not isinstance(payload, dict):
        message = "gateway returned an unexpected JSON shape"
        raise GatewayError(message)
    return Completion(text=_extract_text(payload), usage=_extract_usage(payload))
