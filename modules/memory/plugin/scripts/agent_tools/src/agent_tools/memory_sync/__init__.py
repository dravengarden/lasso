"""Translate Codex facts and audit follower-runtime stores read-only.

Every runtime keeps its own memory store. This package never shares a mutable
store between runtimes: it reads one store, applies deterministic structure and
a bounded model judgment, and emits reviewable output for the other.
"""
