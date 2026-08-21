"""Classify shell commands that cannot mutate a Lasso checkout."""

from __future__ import annotations

import os
import re
import shlex


ASSIGNMENT = re.compile(r"[A-Za-z_][A-Za-z0-9_]*=.*", re.DOTALL)
SHELL_PUNCTUATION = ";&|()<>\n"
SHELL_BOUNDARY = frozenset(";&|()\n")
GENERIC_READ_ONLY = frozenset(
    {
        "[",
        "basename",
        "cat",
        "cmp",
        "cut",
        "date",
        "df",
        "diff",
        "dirname",
        "du",
        "echo",
        "file",
        "free",
        "grep",
        "head",
        "hostname",
        "id",
        "jq",
        "ls",
        "pgrep",
        "printenv",
        "printf",
        "ps",
        "pwd",
        "readlink",
        "realpath",
        "rg",
        "sha256sum",
        "sort",
        "ss",
        "stat",
        "tail",
        "test",
        "tree",
        "tr",
        "type",
        "uname",
        "uniq",
        "wc",
        "which",
        "whoami",
    }
)
GIT_READ_ONLY = frozenset(
    {
        "blame",
        "cat-file",
        "check-attr",
        "check-ignore",
        "count-objects",
        "describe",
        "diff",
        "diff-index",
        "diff-tree",
        "fetch",
        "for-each-ref",
        "grep",
        "help",
        "log",
        "ls-files",
        "ls-remote",
        "ls-tree",
        "merge-base",
        "name-rev",
        "rev-list",
        "rev-parse",
        "shortlog",
        "show",
        "show-ref",
        "status",
        "version",
        "whatchanged",
    }
)


def shell_words(command: str) -> list[str] | None:
    try:
        lexer = shlex.shlex(command, posix=True, punctuation_chars=SHELL_PUNCTUATION)
        lexer.whitespace = " \t\r"
        lexer.whitespace_split = True
        return list(lexer)
    except ValueError:
        return None


def git_read_only(words: list[str]) -> bool:
    index = 1
    options_with_values = {"-C", "-c", "--config-env", "--git-dir", "--namespace", "--work-tree"}
    while index < len(words):
        word = words[index]
        if word in options_with_values:
            index += 2
            continue
        if any(word.startswith(f"{option}=") for option in options_with_values if option.startswith("--")):
            index += 1
            continue
        if word in {"--bare", "--literal-pathspecs", "--no-optional-locks", "--no-replace-objects"}:
            index += 1
            continue
        if word in {"--help", "--version"}:
            return index == len(words) - 1
        break
    if index >= len(words):
        return True
    subcommand = words[index]
    arguments = words[index + 1 :]
    if subcommand == "fetch":
        return "--update-head-ok" not in arguments and not any(
            ":" in argument for argument in arguments if not argument.startswith("--")
        )
    if subcommand in GIT_READ_ONLY:
        return True
    if subcommand == "worktree":
        return not arguments or arguments[0] == "list"
    if subcommand == "remote":
        return not arguments or arguments[0] in {"-v", "get-url", "show"}
    if subcommand == "reflog":
        return not arguments or arguments[0] == "show"
    if subcommand == "tag":
        return not arguments or any(argument in {"-l", "--list"} for argument in arguments)
    if subcommand == "branch":
        listing = {
            "-a", "-r", "-v", "-vv", "--contains", "--format", "--list",
            "--merged", "--no-contains", "--no-merged", "--points-at", "--show-current",
        }
        return not arguments or any(argument.split("=", 1)[0] in listing for argument in arguments)
    return False


def simple_command_read_only(words: list[str]) -> bool:
    while words and ASSIGNMENT.fullmatch(words[0]):
        words = words[1:]
    if not words:
        return True
    if os.path.basename(words[0]) != words[0]:
        return False
    command = words[0]
    if command == "git":
        return git_read_only([command, *words[1:]])
    if command == "command":
        return len(words) >= 2 and words[1] in {"-v", "-V"}
    if command not in GENERIC_READ_ONLY:
        return False
    if command == "rg" and any(word == "--pre" or word.startswith("--pre=") for word in words[1:]):
        return False
    return True


def shell_read_only(command: str) -> bool:
    if "$(" in command or "`" in command:
        return False
    words = shell_words(command)
    if words is None:
        return False
    current: list[str] = []
    index = 0
    while index < len(words):
        word = words[index]
        if word in {"<", ">", ">>", "<&", ">&"}:
            if current and current[-1].isdigit():
                current.pop()
            if index + 1 >= len(words):
                return False
            target = words[index + 1]
            if word == "<":
                index += 2
                continue
            if word in {"<&", ">&"} and (target.isdigit() or target == "-"):
                index += 2
                continue
            if word in {">", ">>"} and target == "/dev/null":
                index += 2
                continue
            return False
        if "<" in word or ">" in word:
            return False
        if word and all(character in SHELL_BOUNDARY for character in word):
            if not simple_command_read_only(current):
                return False
            current = []
        else:
            current.append(word)
        index += 1
    return simple_command_read_only(current)
