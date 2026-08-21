#!/usr/bin/env python3
"""Keep agent mutations out of Lasso stable integration checkouts.

The active agent runtime owns task checkout creation. This hook is deliberately
only a last-line guardrail for the stable checkouts that headless tools must
retain; it is not a worktree manager or a shell sandbox.

A registered linked worktree is never a stable checkout, including when the
runtime places it inside the Lasso root. Registration in `git worktree list`
is the test, so an ordinary directory cannot claim the exemption by name.
"""

from __future__ import annotations

import functools
import json
import os
import pathlib
import re
import subprocess
import sys

from shell_read_only import shell_read_only, shell_words


PATCH_FILE = re.compile(r"^\*\*\* (?:Add|Delete|Update) File: (.+)$", re.MULTILINE)


def deny(reason: str) -> None:
    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": reason,
                }
            },
            separators=(",", ":"),
        )
    )


def git(cwd: pathlib.Path, *args: str) -> str | None:
    completed = subprocess.run(
        ["git", "-C", str(cwd), *args],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True,
    )
    return completed.stdout.strip() if completed.returncode == 0 else None


def _config_path() -> pathlib.Path:
    return pathlib.Path(
        os.environ.get("LASSO_CONFIG")
        or os.environ.get("COLUMBUS_CONFIG")
        or "/etc/lasso/config.json"
    )


@functools.cache
def lasso_root() -> pathlib.Path:
    """Resolve the workspace root without inventing a personal home default."""
    for key in ("LASSO_ROOT", "COLUMBUS_ROOT"):
        value = os.environ.get(key, "").strip()
        if value:
            return pathlib.Path(value).resolve()
    try:
        value = json.loads(_config_path().read_text()).get("paths", {}).get("root")
        if isinstance(value, str) and value:
            return pathlib.Path(value).resolve()
    except (OSError, ValueError):
        pass
    cwd = pathlib.Path.cwd().resolve()
    for directory in (cwd, *cwd.parents):
        if (directory / "project-defs" / "registry.toml").is_file():
            return directory
    # Last resort: empty path that never matches real checkouts.
    return pathlib.Path("/").resolve()


def protected_primary(cwd: pathlib.Path) -> pathlib.Path | None:
    root_text = git(cwd, "rev-parse", "--show-toplevel")
    git_dir = git(cwd, "rev-parse", "--path-format=absolute", "--git-dir")
    common_dir = git(cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
    if not root_text or not git_dir or not common_dir or git_dir != common_dir:
        return None
    root = pathlib.Path(root_text).resolve()
    central = lasso_root()
    if root == central:
        return root
    try:
        relative = root.relative_to(central / "projects")
    except ValueError:
        return None
    if not relative.parts:
        return None
    definition = central / "project-defs" / relative.parts[0] / "project.toml"
    return root if definition.is_file() else None


@functools.cache
def linked_worktrees() -> frozenset[pathlib.Path]:
    """Return the Lasso repository's registered linked worktrees.

    The primary worktree is excluded: it is the stable checkout this hook
    exists to protect. An unreadable or non-Git root yields an empty set, so
    the guardrail keeps its previous behavior instead of failing open.
    """
    central = lasso_root()
    listing = git(central, "worktree", "list", "--porcelain")
    if not listing:
        return frozenset()
    prefix = "worktree "
    roots = set()
    for line in listing.splitlines():
        if not line.startswith(prefix):
            continue
        try:
            root = pathlib.Path(line[len(prefix) :]).resolve()
        except OSError:
            continue
        if root != central:
            roots.add(root)
    return frozenset(roots)


def in_linked_worktree(candidate: pathlib.Path) -> bool:
    """Return whether a resolved path lives inside a registered linked worktree."""
    return any(candidate == root or root in candidate.parents for root in linked_worktrees())


def protected_path(value: str, cwd: pathlib.Path) -> pathlib.Path | None:
    candidate = pathlib.Path(value.strip().strip('"\''))
    if not candidate.is_absolute():
        candidate = cwd / candidate
    try:
        candidate = candidate.resolve()
    except OSError:
        return None
    if in_linked_worktree(candidate):
        return None
    central = lasso_root()
    if candidate == central or central in candidate.parents:
        return central
    return None


def referenced_protected_path(command: str, cwd: pathlib.Path) -> pathlib.Path | None:
    words = shell_words(command)
    if words is None:
        return protected_path(command, cwd)
    roots = (lasso_root(),)
    for root in roots:
        # Catch a protected path that shell splitting hid inside a larger token,
        # then re-check the whole token so a linked worktree keeps its exemption.
        for match in re.finditer(rf"{re.escape(str(root))}(?:/|(?=$|[\s\"';&|)]))", command):
            token = re.split(r"[\s\"';&|()]", command[match.start() :], maxsplit=1)[0]
            if target := protected_path(token, cwd):
                return target
    for word in words:
        candidates = [word]
        if "=" in word:
            candidates.append(word.split("=", 1)[1])
        for candidate in candidates:
            if target := protected_path(candidate.split("#", 1)[0], cwd):
                return target
    return None


def inspect_payload(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return None
    tool_input = payload.get("tool_input", {})
    if not isinstance(tool_input, dict):
        tool_input = {}
    cwd = pathlib.Path(str(tool_input.get("workdir") or payload.get("cwd") or "."))
    tool = payload.get("tool_name")
    command = tool_input.get("command")
    if tool == "Bash" and (not isinstance(command, str) or shell_read_only(command)):
        return None

    stable = protected_primary(cwd)
    if tool in {"Edit", "Write"}:
        file_path = tool_input.get("file_path")
        if not isinstance(file_path, str) or not file_path:
            return (
                f"Could not determine the {tool} target path. Refusing to bypass "
                "the Lasso stable-checkout guardrail."
            )
        if target := protected_path(file_path, cwd):
            return (
                f"{tool} targets Lasso stable integration checkout {target}. Create or resume an isolated task worktree from freshly fetched origin/main, then edit there."
            )
        if stable is not None:
            return (
                f"{stable} is a Lasso stable integration checkout. Resume or create an isolated task worktree from freshly fetched origin/main, then edit there."
            )
        return None
    if tool == "apply_patch":
        if isinstance(command, str):
            paths = PATCH_FILE.findall(command)
            for path in paths:
                if target := protected_path(path, cwd):
                    return (
                        f"Patch targets Lasso stable integration checkout {target}. Create or resume an isolated task worktree from freshly fetched origin/main, then edit there."
                    )
            if paths:
                if stable is not None:
                    return (
                        f"{stable} is a Lasso stable integration checkout. Set the tool working directory to the isolated task worktree before patching it."
                    )
                return None
        if stable is None:
            return None
        return (
            f"{stable} is a Lasso stable integration checkout. Resume or create an isolated task worktree from freshly fetched origin/main, then edit there."
        )
    if not isinstance(command, str):
        return None
    target = stable or referenced_protected_path(command, cwd)
    if target is not None:
        return (
            f"Refusing a shell command that is not provably read-only in stable checkout {target}. Use an isolated task worktree; deploy only a clean commit that contains fresh origin/main and the active machine revision."
        )
    return None


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except (OSError, ValueError):
        return
    if reason := inspect_payload(payload):
        deny(reason)


if __name__ == "__main__":
    main()
