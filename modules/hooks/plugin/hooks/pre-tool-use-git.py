#!/usr/bin/env python3
"""Block high-risk Git and managed-NixOS commands in agent shell calls.

This is a narrow guardrail, not a shell sandbox. It lexes ordinary shell
commands, wrapper invocations, and common nested `sh -c` forms without running
or rewriting the requested command.
"""

from __future__ import annotations

import json
import os
import pathlib
import re
import shlex
import sys
from collections.abc import Iterable, Sequence


CONTROL = {";", ";;", "&", "&&", "|", "||", "(", ")", "{", "}"}
LEADING_KEYWORDS = {"!", "do", "elif", "else", "if", "then", "time", "until", "while"}
ASSIGNMENT = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
RISK_CANDIDATE = re.compile(
    r"(?:^|[^A-Za-z0-9_-])(?:[^\s;&|(){}]*/)?"
    r"(?:git|home-manager|nixos-rebuild|switch-to-configuration)"
    r"(?:[^A-Za-z0-9_-]|$)"
)


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


def tokenize(source: str) -> list[str]:
    lexer = shlex.shlex(source, posix=True, punctuation_chars=";&|(){}")
    lexer.whitespace_split = True
    lexer.commenters = ""
    return list(lexer)


def has_active_backtick(source: str) -> bool:
    """Return whether shell would treat a backtick as command substitution."""
    in_single = False
    in_double = False
    escaped = False
    for char in source:
        if escaped:
            escaped = False
            continue
        if char == "\\" and not in_single:
            escaped = True
            continue
        if char == "'" and not in_double:
            in_single = not in_single
            continue
        if char == '"' and not in_single:
            in_double = not in_double
            continue
        if char == "`" and not in_single:
            return True
    return False


def segments(tokens: Sequence[str]) -> Iterable[list[str]]:
    current: list[str] = []
    for token in tokens:
        if token in CONTROL or all(char in ";&|(){}" for char in token):
            if current:
                yield current
                current = []
            continue
        current.append(token)
    if current:
        yield current


def consume_wrappers(tokens: Sequence[str]) -> int | None:
    index = 0
    while index < len(tokens) and (tokens[index] in LEADING_KEYWORDS or ASSIGNMENT.match(tokens[index])):
        index += 1

    while index < len(tokens):
        command = os.path.basename(tokens[index])
        if command == "env":
            index += 1
            while index < len(tokens):
                token = tokens[index]
                if token == "--":
                    index += 1
                    break
                if token in {"-u", "--unset", "-C", "--chdir"}:
                    index += 2
                    continue
                if token.startswith(("--unset=", "--chdir=")) or token.startswith("-") or ASSIGNMENT.match(token):
                    index += 1
                    continue
                break
            continue
        if command == "sudo":
            index += 1
            value_options = {
                "-C",
                "-D",
                "-R",
                "-T",
                "-g",
                "-h",
                "-p",
                "-u",
                "--chdir",
                "--chroot",
                "--close-from",
                "--command-timeout",
                "--group",
                "--host",
                "--prompt",
                "--user",
            }
            while index < len(tokens):
                token = tokens[index]
                if token == "--":
                    index += 1
                    break
                if token in value_options:
                    index += 2
                    continue
                if token.startswith("-"):
                    index += 1
                    continue
                break
            continue
        if command in {"command", "exec"}:
            if index + 1 < len(tokens) and tokens[index + 1] in {"-v", "-V"}:
                return None
            index += 1
            while index < len(tokens) and tokens[index].startswith("-"):
                index += 1
            continue
        if command == "timeout":
            index += 1
            while index < len(tokens):
                token = tokens[index]
                if token == "--":
                    index += 1
                    break
                if token in {"-k", "--kill-after", "-s", "--signal"}:
                    index += 2
                    continue
                if token.startswith(("--kill-after=", "--signal=")) or token.startswith("-"):
                    index += 1
                    continue
                break
            if index >= len(tokens):
                return None
            index += 1  # duration
            continue
        if command == "nice":
            index += 1
            while index < len(tokens):
                token = tokens[index]
                if token == "--":
                    index += 1
                    break
                if token in {"-n", "--adjustment"}:
                    index += 2
                    continue
                if token.startswith("--adjustment=") or re.fullmatch(r"-\d+", token):
                    index += 1
                    continue
                break
            continue
        break

    return index if index < len(tokens) else None


def git_subcommand(tokens: Sequence[str], git_index: int) -> tuple[str, list[str]] | None:
    index = git_index + 1
    value_options = {"-C", "-c", "--config-env", "--git-dir", "--namespace", "--super-prefix", "--work-tree"}
    while index < len(tokens):
        token = tokens[index]
        if token == "--":
            index += 1
            break
        if token in value_options:
            index += 2
            continue
        if token.startswith(("--config-env=", "--git-dir=", "--namespace=", "--super-prefix=", "--work-tree=")):
            index += 1
            continue
        if token.startswith("-c") and token != "-c":
            index += 1
            continue
        if token.startswith("-"):
            index += 1
            continue
        break
    if index >= len(tokens):
        return None
    return tokens[index], list(tokens[index + 1 :])


def git_global_risk(tokens: Sequence[str], git_index: int) -> str | None:
    """Reject inline aliases before git_subcommand skips global options."""
    index = git_index + 1
    value_options = {"-C", "--git-dir", "--namespace", "--super-prefix", "--work-tree"}
    while index < len(tokens):
        token = tokens[index]
        if token == "--":
            return None
        if token in {"-c", "--config-env"}:
            if index + 1 < len(tokens) and tokens[index + 1].split("=", 1)[0].lower().startswith("alias."):
                return "Inline Git aliases are disabled because they can hide destructive subcommands from the multi-machine safety check."
            index += 2
            continue
        if token.startswith("-c") and token != "-c":
            if token[2:].split("=", 1)[0].lower().startswith("alias."):
                return "Inline Git aliases are disabled because they can hide destructive subcommands from the multi-machine safety check."
            index += 1
            continue
        if token.startswith("--config-env="):
            if token.removeprefix("--config-env=").split("=", 1)[0].lower().startswith("alias."):
                return "Inline Git aliases are disabled because they can hide destructive subcommands from the multi-machine safety check."
            index += 1
            continue
        if token in value_options:
            index += 2
            continue
        if token.startswith(("--git-dir=", "--namespace=", "--super-prefix=", "--work-tree=")):
            index += 1
            continue
        if token.startswith("-"):
            index += 1
            continue
        return None
    return None


def has_short_flag(args: Sequence[str], flag: str) -> bool:
    return any(
        arg.startswith("-")
        and not arg.startswith("--")
        and flag in arg[1:]
        for arg in args
    )


def git_risk(subcommand: str, args: Sequence[str]) -> str | None:
    if subcommand == "pull":
        return "Raw git pull is disabled for multi-machine work. Fetch first, inspect dirty/ahead/behind state, then use an explicit fast-forward, rebase, or merge in the owning worktree."
    if subcommand == "push":
        if (
            has_short_flag(args, "f")
            or any(arg == "--force" or arg.startswith("--force-with-lease") or arg == "--force-if-includes" for arg in args)
            or any(arg.startswith("+") for arg in args)
        ):
            return "Force-push is disabled because another machine or agent may own the remote ref. Fetch and reconcile without rewriting shared history."
        if "--mirror" in args or "--prune" in args:
            return "git push --mirror and --prune delete remote refs that are missing locally, including branches another machine still owns. Push explicit refspecs instead."
        if (
            has_short_flag(args, "d")
            or "--delete" in args
            or any(arg.startswith("--delete=") for arg in args)
            or any(arg.startswith(":") and len(arg) > 1 for arg in args)
        ):
            return "Remote branch deletion requires an explicit user-operated path; it is blocked in ordinary agent shell calls."
    if subcommand == "reset" and "--soft" not in args:
        # --soft only moves HEAD: the index and working tree survive, and any
        # un-committed revision stays reachable through the reflog.
        return "git reset is disabled because it can discard index and working-tree state; use git reset --soft to move HEAD only, or restore --staged to unstage."
    if subcommand == "clean" and not ("--dry-run" in args or has_short_flag(args, "n")):
        return "Destructive git clean is disabled. Inspect ignored and untracked files with --dry-run and preserve machine-local work."
    if subcommand == "checkout":
        return "Ambiguous git checkout is disabled because path forms can overwrite working-tree changes; use git switch for branches."
    if subcommand == "restore":
        staged_only = "--staged" in args or has_short_flag(args, "S")
        touches_worktree = "--worktree" in args or has_short_flag(args, "W")
        if not staged_only or touches_worktree:
            return "Working-tree git restore is disabled because it can discard another agent's changes; index-only --staged remains allowed."
    if subcommand == "switch" and (
        "--discard-changes" in args
        or "--force" in args
        or "--force-create" in args
        or "--orphan" in args
        or has_short_flag(args, "f")
        or has_short_flag(args, "C")
    ):
        return "Forced or orphan git switch is disabled because it can overwrite checkout state or an existing branch; use a non-destructive switch or a fresh worktree."
    if subcommand == "branch" and (
        "--force" in args
        or has_short_flag(args, "f")
        or has_short_flag(args, "D")
        or has_short_flag(args, "M")
        or has_short_flag(args, "C")
    ):
        return "Forced local branch movement, overwrite, or deletion is disabled because a branch may contain unpublished work."
    if subcommand == "stash" and args and args[0] in {"clear", "drop"}:
        return "Destructive stash deletion is disabled because stashes may contain unpublished machine-local work."
    if subcommand in {"merge", "rebase"} and "--autostash" in args:
        return "Automatic stashing is disabled; inspect and preserve the dirty checkout before integrating remote changes."
    if subcommand == "worktree" and args and args[0] == "remove" and ("--force" in args or has_short_flag(args, "f")):
        return "Forced worktree removal is disabled; the active runtime owns task worktree lifecycle and unpublished changes must be preserved."
    return None


def _nixos_hosts() -> set[str]:
    """Hosts that opt into the committed-source NixOS transaction fence."""
    config = pathlib.Path(
        os.environ.get("LASSO_CONFIG")
        or os.environ.get("COLUMBUS_CONFIG")
        or "/etc/lasso/config.json"
    )
    try:
        data = json.loads(config.read_text())
    except (OSError, ValueError):
        return set()
    machine = data.get("machine", {})
    if not isinstance(machine, dict):
        return set()
    hosts = machine.get("nixos_hosts", [])
    if isinstance(hosts, list):
        return {item for item in hosts if isinstance(item, str) and item}
    return set()


def nixos_risk(command: str, args: Sequence[str]) -> str | None:
    machine = os.environ.get("LASSO_MACHINE_ID") or os.environ.get("COLUMBUS_MACHINE_ID", "")
    if not machine:
        config = pathlib.Path(
            os.environ.get("LASSO_CONFIG")
            or os.environ.get("COLUMBUS_CONFIG")
            or "/etc/lasso/config.json"
        )
        try:
            value = json.loads(config.read_text()).get("machine", {}).get("id")
            machine = value if isinstance(value, str) else ""
        except (OSError, ValueError):
            machine = ""
    if not machine or machine not in _nixos_hosts():
        return None
    if command == "nixos-rebuild" and any(
        argument in {"boot", "build", "switch", "test"} for argument in args
    ):
        return (
            "Direct nixos-rebuild build/boot/test/switch bypasses the committed-source "
            "host machine transaction. Use an isolated task worktree and the host-owned "
            "build/activate contract."
        )
    if command == "switch-to-configuration" and any(
        argument in {"boot", "switch", "test"} for argument in args
    ):
        return (
            "Direct switch-to-configuration bypasses target serialization, provenance, "
            "health checks, rollback, and receipt. Use the host-owned activate contract."
        )
    if command == "home-manager" and "switch" in args:
        return (
            "This host integrates Home Manager into the NixOS closure. Build and "
            "activate that complete closure through the host-owned contract."
        )
    return None


def inspect_tokens(tokens: Sequence[str]) -> str | None:
    for segment in segments(tokens):
        index = consume_wrappers(segment)
        if index is None:
            continue
        command = os.path.basename(segment[index])
        args = segment[index + 1 :]
        if command in {"bash", "dash", "fish", "ksh", "sh", "zsh"}:
            for option_index, option in enumerate(args):
                if re.fullmatch(r"-[^-]*c[^-]*", option) and option_index + 1 < len(args):
                    source_index = option_index + 1
                    if args[source_index] == "--":
                        source_index += 1
                    if source_index >= len(args):
                        break
                    risk = inspect_source(args[source_index])
                    if risk:
                        return risk
                    break
        if command == "eval" and args:
            risk = inspect_source(" ".join(args))
            if risk:
                return risk
        if command == "nix" and args and args[0] == "develop":
            for separator in ("-c", "--command"):
                if separator in args:
                    nested_index = args.index(separator) + 1
                    risk = inspect_tokens(args[nested_index:])
                    if risk:
                        return risk
                    break
        risk = nixos_risk(command, args)
        if risk:
            return risk
        if command != "git":
            continue
        risk = git_global_risk(segment, index)
        if risk:
            return risk
        parsed = git_subcommand(segment, index)
        if parsed is None:
            continue
        subcommand, git_args = parsed
        risk = git_risk(subcommand, git_args)
        if risk:
            return risk
    return None


def inspect_source(source: str) -> str | None:
    if not RISK_CANDIDATE.search(source):
        return None
    if has_active_backtick(source):
        return "Git-related backtick command substitution is disabled because it cannot be inspected safely; use explicit commands or a parseable $(...) form."
    try:
        return inspect_tokens(tokenize(source))
    except ValueError:
        return "A Git-related shell command could not be parsed safely; split it into explicit fetch/status/integration commands and retry."


def inspect_payload(payload: object) -> str | None:
    if not isinstance(payload, dict):
        return None
    tool_input = payload.get("tool_input", {})
    if not isinstance(tool_input, dict):
        return None
    command = tool_input.get("command", "")
    if not isinstance(command, str):
        return None
    return inspect_source(command)


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, TypeError) as error:
        deny(f"Invalid PreToolUse hook input: {error}")
        return 0
    if risk := inspect_payload(payload):
        deny(risk)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
