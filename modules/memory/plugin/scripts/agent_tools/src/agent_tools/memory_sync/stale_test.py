from agent_tools.memory_sync.facts import RawFact
from agent_tools.memory_sync.stale import claude_memory_dirs, grok_memory_locations, inspect, referenced_paths, scan


def test_referenced_paths_finds_backticked_and_bare_absolute_paths():
    text = "Use `/etc/nixos/flake.lock` rather than /home/draven/columbus/machines/flake.lock."
    assert referenced_paths(text) == ["/etc/nixos/flake.lock", "/home/draven/columbus/machines/flake.lock"]


def test_referenced_paths_ignores_relative_and_volatile_paths():
    assert referenced_paths("edit machines/flake.nix and check /proc/1/exe") == []


def test_a_relative_path_tail_is_not_read_as_absolute():
    # Both were reported missing by a real run before the left boundary existed.
    assert referenced_paths("read `web/src/mdlive/PITFALLS.md` first") == []
    assert referenced_paths("artifacts land under code/tests/deployment") == []


def test_referenced_paths_trims_sentence_punctuation():
    assert referenced_paths("see /etc/nixos/configuration.nix.") == ["/etc/nixos/configuration.nix"]


def test_inspect_reports_a_missing_path(tmp_path):
    missing = tmp_path / "gone" / "file.md"
    fact = RawFact(path=tmp_path / "f.md", name="f", description="", body=f"Read `{missing}`.")
    finding = inspect(fact)
    assert finding is not None
    assert finding.missing == [str(missing)]


def test_inspect_is_silent_when_every_path_exists(tmp_path):
    present = tmp_path / "present.md"
    present.write_text("x", encoding="utf-8")
    fact = RawFact(path=tmp_path / "f.md", name="f", description="", body=f"Read `{present}`.")
    assert inspect(fact) is None


def test_scan_never_removes_anything(tmp_path):
    directory = tmp_path / "facts"
    directory.mkdir()
    target = directory / "f.md"
    target.write_text("---\nname: f\ndescription: d\n---\n\nRead `/nonexistent/path/file.md`.\n", encoding="utf-8")
    findings = scan([directory])
    assert [finding.name for finding in findings] == ["f"]
    assert target.is_file()


def test_store_directory_discovery_prefers_runtime_project_stores(tmp_path):
    claude = tmp_path / "claude"
    (claude / "projects" / "-home-draven-columbus" / "memory").mkdir(parents=True)
    assert claude_memory_dirs(claude) == [claude / "projects" / "-home-draven-columbus" / "memory"]

    grok = tmp_path / "grok"
    global_memory = grok / "memory" / "MEMORY.md"
    workspace_memory = grok / "memory" / "cowboy-a1b2c3d4" / "MEMORY.md"
    sessions = workspace_memory.parent / "sessions"
    sessions.mkdir(parents=True)
    global_memory.parent.mkdir(parents=True, exist_ok=True)
    global_memory.write_text("global", encoding="utf-8")
    workspace_memory.write_text("workspace", encoding="utf-8")
    assert grok_memory_locations(grok) == [global_memory, workspace_memory, sessions]


def test_scan_reads_grok_memory_files_without_rewriting_them(tmp_path):
    memory = tmp_path / "MEMORY.md"
    memory.write_text("Retired checkout: `/missing/grok/worktree`.", encoding="utf-8")
    findings = scan([memory])
    assert [finding.path for finding in findings] == [memory]
    assert memory.read_text(encoding="utf-8") == "Retired checkout: `/missing/grok/worktree`."


def test_missing_stores_are_not_an_error(tmp_path):
    assert claude_memory_dirs(tmp_path) == []
    assert grok_memory_locations(tmp_path) == []
    assert scan([tmp_path / "absent"]) == []
