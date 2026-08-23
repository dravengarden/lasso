"""CLI contract tests for the dependency security report."""

from __future__ import annotations

from typing import TYPE_CHECKING

from typer.testing import CliRunner

from .models import ToolEntry, ToolsManifest, UnifiedRow
from .report import app

if TYPE_CHECKING:
    from pathlib import Path


def test_analyze_help_is_codex_first_with_claude_compatibility() -> None:
    """The review workflow should advertise Codex before Claude compatibility."""
    result = CliRunner().invoke(app, ["analyze", "--help"])

    assert result.exit_code == 0
    # typer's rich table wraps option text at the terminal width, so the
    # runtime enum may be split across lines; assert tokens, not formatting.
    assert "codex" in result.stdout
    assert "claude-code" in result.stdout
    assert result.stdout.index("codex") < result.stdout.index("claude-code")
    assert "actual agent model id" in result.stdout
    assert "gemini" not in result.stdout.lower()


def test_pending_table_uses_project_scanner_denominator(tmp_path: Path) -> None:
    scan_dir = tmp_path / "scan"
    merged = scan_dir / "merged"
    merged.mkdir(parents=True)
    row = UnifiedRow(
        project="rust-only",
        package="demo",
        version="1.0.0",
        cluster_id="RUSTSEC-TEST",
        scanners=["osv-scanner"],
        scanner_count=1,
    )
    (merged / "unified.jsonl").write_text(row.model_dump_json() + "\n")
    (scan_dir / "confirmations.jsonl").write_text("")
    manifest = ToolsManifest(
        tools={
            "osv-scanner": ToolEntry(),
            "trivy": ToolEntry(),
            "grype": ToolEntry(),
        },
        projects_scanned=["rust-only"],
        project_scanners={"rust-only": ["osv-scanner", "trivy", "grype"]},
    )
    (scan_dir / "tools.json").write_text(manifest.model_dump_json() + "\n")

    result = CliRunner().invoke(
        app,
        ["pending", "--scan-dir", str(scan_dir), "--all", "--format", "table"],
    )

    assert result.exit_code == 0
    assert "1/3" in result.stdout
