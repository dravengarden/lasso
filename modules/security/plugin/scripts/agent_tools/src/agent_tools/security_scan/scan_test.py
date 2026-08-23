"""Scanner execution-contract tests."""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from .models import ScannerVersions
from .normalize import normalize
from .scan import _require_json_output, _run_capture, _run_stderr_only, _tools_manifest

if TYPE_CHECKING:
    from collections.abc import Callable
    from pathlib import Path


def test_manifest_records_only_verified_project_scanners() -> None:
    versions = ScannerVersions(
        osv_scanner="2.3.3",
        pip_audit="2.10.1",
        trivy="0.72.0",
        grype="0.115.0",
    )

    manifest = _tools_manifest(
        ["rust-only"],
        versions,
        {"rust-only": ["osv-scanner", "trivy", "grype"]},
    )

    assert list(manifest.tools) == ["osv-scanner", "trivy", "grype"]
    assert manifest.scanner_total_for("rust-only") == 3
    assert "pip-audit" not in manifest.scanners_for_project("rust-only")


def test_scanner_output_requires_expected_schema(tmp_path: Path) -> None:
    output = tmp_path / "scanner.json"
    stderr = tmp_path / "scanner.stderr"
    stderr.write_text("scanner failed\n")
    output.write_text(json.dumps({"error": "database unavailable"}))

    with pytest.raises(RuntimeError, match="expected list field 'results'"):
        _require_json_output("osv-scanner", output, stderr, "results")


def test_normalize_omits_scanners_without_output(tmp_path: Path) -> None:
    (tmp_path / "raw/pip-audit").mkdir(parents=True)

    stats = normalize(tmp_path)

    assert stats.per_scanner == {}
    assert stats.total == 0


@pytest.mark.parametrize("runner", [_run_capture, _run_stderr_only])
def test_scanner_invocation_removes_stale_output(
    tmp_path: Path,
    runner: Callable[[list[str], Path, Path], int],
) -> None:
    output = tmp_path / "scanner.json"
    stderr = tmp_path / "scanner.stderr"
    output.write_text(json.dumps({"results": []}))

    result = runner(["/definitely-missing-columbus-scanner"], output, stderr)

    assert result == 127
    assert not output.exists()
    assert "failed to invoke" in stderr.read_text()
