#!/usr/bin/env python3
"""Run Lasso PreToolUse policies in one interpreter process."""

from __future__ import annotations

import json
import sys

from shell_read_only import shell_read_only


def load_policy(filename: str) -> object:
    import importlib.util
    import pathlib

    path = pathlib.Path(__file__).with_name(filename)
    spec = importlib.util.spec_from_file_location(filename.removesuffix(".py"), path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"could not load hook policy: {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def deny(reason: str) -> None:
    print(
        json.dumps(
            {
                "decision": "deny",
                "reason": reason,
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                }
            },
            separators=(",", ":"),
        )
    )


def normalize_payload(payload: object) -> dict[str, object]:
    """Normalize Claude/Codex and Grok file-hook envelopes and tool names."""
    if not isinstance(payload, dict):
        return {}
    normalized = dict(payload)
    if "tool_name" not in normalized and "toolName" in normalized:
        normalized["tool_name"] = normalized["toolName"]
    if "tool_input" not in normalized and "toolInput" in normalized:
        normalized["tool_input"] = normalized["toolInput"]
    aliases = {
        "run_terminal_command": "Bash",
        "search_replace": "Edit",
        "write_file": "Write",
    }
    tool_name = normalized.get("tool_name")
    if isinstance(tool_name, str):
        normalized["tool_name"] = aliases.get(tool_name, tool_name)
    tool_input = normalized.get("tool_input")
    if isinstance(tool_input, dict) and "file_path" not in tool_input and "path" in tool_input:
        normalized["tool_input"] = {**tool_input, "file_path": tool_input["path"]}
    return normalized


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, TypeError) as error:
        deny(f"Invalid PreToolUse hook input: {error}")
        return 0
    payload = normalize_payload(payload)

    tool_input = payload.get("tool_input", {})
    if payload.get("tool_name") == "Bash" and isinstance(tool_input, dict):
        command = tool_input.get("command")
        if isinstance(command, str) and shell_read_only(command):
            return 0

    worktree_policy = load_policy("pre-tool-use-worktree.py")
    if reason := worktree_policy.inspect_payload(payload):
        deny(reason)
        return 0

    if payload.get("tool_name") == "Bash":
        git_policy = load_policy("pre-tool-use-git.py")
        if reason := git_policy.inspect_payload(payload):
            deny(reason)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
