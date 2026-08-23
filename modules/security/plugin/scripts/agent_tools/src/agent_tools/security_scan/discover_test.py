from typing import TYPE_CHECKING

from .discover import default_branch, discover, project_checkout
from .iocs import _resolve_worktree

if TYPE_CHECKING:
    from pathlib import Path


def _register(root: Path, name: str, *, kind: str = "external", branch: str = "main") -> Path:
    project_def = root / "project-defs" / name
    project_def.mkdir(parents=True)
    if kind == "external":
        body = f'kind = "external"\nrepo = "https://example.invalid/{name}.git"\ndefault_branch = "{branch}"\n'
        checkout = root / "projects" / name
    else:
        body = f'kind = "{kind}"\n'
        checkout = root / "projects" / name
    (project_def / "project.toml").write_text(body)
    return checkout


def test_resolves_registered_stable_checkout_without_bare_head(tmp_path: Path) -> None:
    checkout = _register(tmp_path, "alpha", branch="trunk")
    checkout.mkdir(parents=True)
    (checkout / "pyproject.toml").write_text("[project]\nname = 'alpha'\n")

    assert default_branch(tmp_path, "alpha") == "trunk"
    assert project_checkout(tmp_path, "alpha") == ("trunk", checkout)
    assert _resolve_worktree(tmp_path, "alpha") == checkout
    [entry] = discover(tmp_path)
    assert entry.present is True
    assert entry.worktree == "projects/alpha"


def test_missing_checkout_routes_to_setup(tmp_path: Path) -> None:
    _register(tmp_path, "alpha")

    [entry] = discover(tmp_path)
    assert entry.present is False
    assert entry.error == "not cloned locally (run `harness-cli setup --only=alpha`)"


def test_subdir_project_resolves_in_tree_path(tmp_path: Path) -> None:
    checkout = _register(tmp_path, "docs", kind="subdir")
    checkout.mkdir(parents=True)

    assert project_checkout(tmp_path, "docs") == (None, checkout)
    assert _resolve_worktree(tmp_path, "docs") == checkout


def test_discovers_rust_and_go_lockfiles(tmp_path: Path) -> None:
    checkout = _register(tmp_path, "systems")
    checkout.mkdir(parents=True)
    (checkout / "Cargo.lock").write_text("version = 4\n")
    (checkout / "go.sum").write_text("example.invalid/module v1.0.0 h1:hash\n")

    [entry] = discover(tmp_path)
    assert entry.languages == ["rust", "go"]
    assert entry.deps.cargo_lock == ["Cargo.lock"]
    assert entry.deps.go_sum == ["go.sum"]
    assert entry.scannable is True
