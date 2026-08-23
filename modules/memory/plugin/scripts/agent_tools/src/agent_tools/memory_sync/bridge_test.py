import os
import subprocess
from pathlib import Path

BRIDGE = Path(__file__).parents[4] / "memory-bridge"


def run_bridge(tmp_path: Path, left: str, right: str) -> str:
    ordinary = tmp_path / "ordinary"
    deepseek = tmp_path / "deepseek"
    relative = Path("memories/extensions/ad_hoc/notes/shared.md")
    (ordinary / relative).parent.mkdir(parents=True)
    (deepseek / relative).parent.mkdir(parents=True)
    (ordinary / relative).write_text(left, encoding="utf-8")
    (deepseek / relative).write_text(right, encoding="utf-8")
    environment = {
        **os.environ,
        "CODEX_A": str(ordinary),
        "CODEX_B": str(deepseek),
        "LOCK_DIR": str(tmp_path / "locks"),
    }
    result = subprocess.run(  # noqa: S603 - fixed repository-owned helper
        [BRIDGE, "--propagate-updates"],
        check=False,
        capture_output=True,
        text=True,
        env=environment,
    )
    assert result.returncode == 0, result.stderr
    return result.stdout


def test_bridge_ignores_its_translation_provenance(tmp_path: Path) -> None:
    source = "# Durable fact\n\nThe fact is shared.\n"
    translated = source + "\n- Recorded by codex; translated by the Lasso Memory plugin.\n"

    assert "differs" not in run_bridge(tmp_path, source, translated)


def test_bridge_reports_a_real_same_name_difference(tmp_path: Path) -> None:
    output = run_bridge(tmp_path, "# Fact\n\nleft\n", "# Fact\n\nright\n")

    assert "differs  codex:notes/shared.md" in output
