"""Discover harness projects with scannable dependency files.

Pure library module — no CLI surface. The orchestrator (`scan.py`)
imports `discover`, `harness_root_default`, and the pydantic models in
`models.py`.

Behavior:
- Globs `project-defs/<name>/project.toml` for registered project names
  (the directory set is the registry — no aggregator file).
- For each project, walks its stable local checkout to find
  Python (`requirements*.txt`, `pyproject.toml`, `uv.lock`), npm
  (`package-lock.json`, `package.json`), Rust (`Cargo.lock`), and Go (`go.sum`)
  manifests.
"""

from __future__ import annotations

import re
import tomllib
from pathlib import Path
from typing import TYPE_CHECKING

from .models import DepFiles, ProjectEntry

if TYPE_CHECKING:
    from collections.abc import Iterable


SKIP_DIRS: frozenset[str] = frozenset(
    {
        ".venv",
        "node_modules",
        ".cache",
        ".direnv",
        ".git",
        ".julia",
        "result",
        "dist",
        "build",
        ".build",
        "target",
        ".pytest_cache",
        "__pycache__",
    }
)
# Also catches `.venv.uv-managed.bak/` and similar suffix variants.
SKIP_PREFIXES: tuple[str, ...] = (".venv",)


def harness_root_default() -> Path:
    """Find the harness root by walking up to project-defs/registry.toml."""
    p = Path.cwd().resolve()
    while p != p.parent:
        if (p / "project-defs" / "registry.toml").exists():
            return p
        p = p.parent
    return Path.cwd()


def read_project_registry(harness_root: Path) -> list[str]:
    """Return project names from directories carrying project.toml.

    The directory set IS the registry — there is no aggregator file. A project
    is exactly a directory under project-defs/ carrying a project.toml.
    """
    project_defs = harness_root / "project-defs"
    if not project_defs.is_dir():
        return []
    return sorted(d.name for d in project_defs.iterdir() if d.is_dir() and (d / "project.toml").is_file())


def default_branch(harness_root: Path, project: str) -> str | None:
    """Return the registered stable checkout branch."""
    project_file = harness_root / "project-defs" / project / "project.toml"
    try:
        value = tomllib.loads(project_file.read_text()).get("default_branch")
    except OSError, tomllib.TOMLDecodeError:
        return None
    return value if isinstance(value, str) and value else None


def project_checkout(harness_root: Path, project: str) -> tuple[str | None, Path]:
    """Resolve the stable ordinary checkout or in-tree project path."""
    project_file = harness_root / "project-defs" / project / "project.toml"
    try:
        config = tomllib.loads(project_file.read_text())
    except OSError, tomllib.TOMLDecodeError:
        config = {}
    base = harness_root / "projects" / project
    if config.get("kind") != "external":
        return None, base
    branch = default_branch(harness_root, project)
    return branch, base


def _is_skipped_part(part: str) -> bool:
    return part in SKIP_DIRS or any(part.startswith(pre) for pre in SKIP_PREFIXES)


def find_dep_files(worktree: Path) -> DepFiles:
    """Categorize dependency files inside `worktree` (paths relative to it)."""
    out = DepFiles()
    if not worktree.exists():
        return out
    req_re = re.compile(r"requirements([_-].*)?\.txt")
    exact_files = {
        "pyproject.toml": out.pyproject_toml,
        "uv.lock": out.uv_lock,
        "package-lock.json": out.package_lock_json,
        "package.json": out.package_json,
        "Cargo.lock": out.cargo_lock,
        "go.sum": out.go_sum,
    }
    for path in worktree.rglob("*"):
        if not path.is_file():
            continue
        if any(_is_skipped_part(part) for part in path.relative_to(worktree).parts):
            continue
        rel = str(path.relative_to(worktree))
        name = path.name
        if req_re.fullmatch(name):
            out.requirements_txt.append(rel)
        elif name in exact_files:
            exact_files[name].append(rel)
    for paths in [out.requirements_txt, *exact_files.values()]:
        paths.sort()
    return out


def discover(
    harness_root: Path,
    projects_filter: Iterable[str] | None = None,
) -> list[ProjectEntry]:
    """Return project entries with default branch + dep-file inventory."""
    all_projects = read_project_registry(harness_root)
    selected = list(projects_filter) if projects_filter is not None else all_projects
    out: list[ProjectEntry] = []
    for name in selected:
        if name not in all_projects:
            out.append(
                ProjectEntry(
                    project=name,
                    registered=False,
                    error="no project-defs/<name>/project.toml",
                )
            )
            continue
        branch, checkout = project_checkout(harness_root, name)
        if not checkout.exists():
            err = f"not cloned locally (run `harness-cli setup --only={name}`)"
            out.append(ProjectEntry(project=name, registered=True, error=err))
            continue
        deps = find_dep_files(checkout)
        languages: list[str] = []
        if deps.has_python():
            languages.append("python")
        if deps.has_npm():
            languages.append("npm")
        if deps.has_rust():
            languages.append("rust")
        if deps.has_go():
            languages.append("go")
        out.append(
            ProjectEntry(
                project=name,
                registered=True,
                branch=branch,
                present=checkout.exists(),
                worktree=str(checkout.relative_to(harness_root)),
                languages=languages,
                deps=deps,
                scannable=bool(languages),
            )
        )
    return out
