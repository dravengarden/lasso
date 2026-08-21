# Rust development standard

This is the default for first-party Rust projects in a Lasso workspace. A
closer repository guide may tighten it or document a justified exception. The
owning repository keeps its toolchain, lockfiles, commands, and gates; Lasso
does not execute builds on its behalf.

The goal is a short edit-to-feedback loop without weakening correctness,
supply-chain policy, or reproducibility. Optimize from measurements. A faster
clean build that makes ordinary incremental checks slower is not an
improvement.

## Required project contract

Every maintained Rust repository must provide these project-owned operations,
normally as `just` recipes:

- `check` or `verify`: the complete local gate;
- `dependencies`: `cargo deny check` plus `cargo machete --with-metadata`;
- `test-fast`: the shortest representative inner-loop test command;
- `build-cached`: an explicitly opt-in sccache build experiment;
- `cache-stats`: `sccache --show-stats`.

The complete gate must check formatting, run Clippy over every maintained
target and feature, execute dependency policy, run tests, and build the shipped
artifacts. Use `--locked`. Keep GitHub CI optional; the authoritative gate is
local and deterministic.

If a repository contains independent Cargo workspaces, audit and test each
one. Native, Wasm, and eBPF manifests do not inherit the root workspace merely
because they share a Git repository.

## Toolchain and manifests

- Pin stable Rust in `rust-toolchain.toml` and construct the Nix shell from that
  file. Do not probe or repair the host Rust installation.
- Set `package.rust-version` to the supported compiler policy and verify that
  it agrees with the pinned toolchain. First-party applications normally track
  the pinned stable compiler rather than claiming an untested historical MSRV.
- New crates use edition 2024. Virtual workspaces set `resolver = "3"`
  explicitly; edition 2024 packages receive the Rust-version-aware resolver
  automatically.
- Workspace members inherit edition, Rust version, license, publication
  posture, and lint tables where Cargo supports inheritance.
- Set `publish = false` for private applications and internal crates. Public
  crates must provide complete package metadata and exact versions alongside
  path dependencies.
- Commit every lockfile used to build an application, native shell, plugin, or
  independent workspace.

Do not upgrade an edition by changing the manifest alone. Run
`cargo fix --edition` on all targets and features while still on the old
edition, inspect every remaining compatibility lint, then change the edition,
format, and run the complete gate. Changes to temporary scopes, `if let`
lifetimes, environment mutation, locks, channels, and destructor order require
manual semantic review. Compilation success is not proof that their behavior
is unchanged.

Rust 2024 makes process-environment mutation unsafe. Do not wrap `set_var` or
`remove_var` in an unsafe block merely to keep a parallel test compiling.
Separate environment lookup from configuration parsing, inject a local reader
or map in tests, and leave production code responsible only for read access.

## Lint baseline

Use manifest lint tables so editors and every Cargo invocation share policy:

```toml
[lints.rust]
warnings = "deny"
unsafe_code = "forbid"

[lints.clippy]
all = { level = "warn", priority = -1 }
cargo = { level = "warn", priority = -1 }
pedantic = { level = "warn", priority = -1 }
dbg_macro = "deny"
todo = "deny"
unimplemented = "deny"
```

`unsafe_code = "deny"` is acceptable where unsafe is intrinsic, such as eBPF,
FFI, or a post-fork pre-exec child. Each unsafe block must state the invariant
that makes it sound. Do not enable the whole Clippy `restriction` group: its
lints are intentionally selective and can contradict one another.

The aggregate lint command ends with `-- -D warnings`. Prefer the `pedantic`
baseline for new projects. Existing projects may ratchet it module by module;
an allow must be narrow and explain why the code is clearer or safer than the
suggestion.

## Dependency policy

`cargo-deny` is a mandatory local gate. Configure all four checks:

- advisories fail unless one exact advisory has a documented upstream blocker;
- licenses are an explicit allowlist;
- unknown registries and Git sources fail;
- wildcard dependencies and unreviewed duplicate versions fail.

An advisory exception names the exact RustSec ID, why no maintained compatible
replacement exists, and the condition for removal. Never use a subtree skip to
hide advisories: duplicate-version skips and advisory ignores solve different
problems. Keep duplicate skips at ecosystem bridge roots such as a framework or
SDK, then periodically re-evaluate them.

Audit every shipped target graph. A Tauri application that ships on macOS and
iOS checks both `aarch64-apple-darwin` and `aarch64-apple-ios`; Linux-only GTK
dependencies are not evidence about the Apple graph. Independent eBPF, plugin,
and native workspaces receive their own manifest invocation even when they can
reuse the repository's policy file.

Run `cargo machete --with-metadata` for unused direct dependencies. Declare a
metadata exception only for a dependency consumed by generated code, a build
script, a proc macro, or another use the analyzer cannot observe. Remove a
dependency instead of suppressing a real finding.

Update dependencies in small coherent groups. After an update, inspect the
lockfile for new major-version bridges and rerun the complete gate. Protocol,
database, cryptography, RPC, serialization, and file-format upgrades also need
a representative live or compatibility fixture.

## Fast local builds

Keep Cargo's incremental compilation enabled for ordinary development. A good
default for service and CLI projects is:

```toml
[profile.dev]
debug = "line-tables-only"
```

Do not globally add optimization, LTO, or a low codegen-unit count to the dev
profile. Those trade runtime speed for slower feedback. Tune a specific hot
dependency with a package override only after a development workload proves it
is necessary.

Use `cargo check` for the edit loop, nextest for parallel test binaries when it
materially helps, and the normal `cargo test` or an explicit doc-test step in
the final gate. Nextest does not run doctests.

### sccache is opt-in

sccache can help clean rebuilds at stable paths and shared dependency graphs.
Rust cache entries require an invocation whose
`--emit` includes `link`; `cargo check` emits metadata without link and is not a
valid cache-reuse benchmark. sccache also cannot cache linking, proc-macro
artifacts, or incrementally compiled crates. Therefore:

- do not set a machine-global `rustc-wrapper`;
- keep the normal incremental build path unchanged;
- expose a separate `build-cached` recipe using `RUSTC_WRAPPER=sccache
  CARGO_INCREMENTAL=0 cargo build ...`;
- when Cargo timings identify a native C or C++ build script as a material
  bottleneck, also route `CC` and `CXX` through sccache in that opt-in recipe
  (for example, `CC='sccache cc' CXX='sccache c++'`); keep the repository's
  Nix-provided compiler behind the wrapper and require observed C/C++ hits;
- use the same locked workspace, target, feature scope, workspace path, and
  target path when establishing a baseline;
- inspect `cache-stats`; a configured wrapper with mostly non-cacheable calls
  is overhead, not an optimization;
- treat new target/worktree paths as separate cases: path normalization does
  not guarantee Rust cache-key reuse, so require observed hits before claiming
  cross-path reuse;

Retain sccache as the default only if representative measurements show a useful
improvement. On a single hot worktree Cargo incremental compilation is often
faster.

### Bound long-lived target caches

Cargo retains intermediate artifacts for old feature, profile, target, and
compiler combinations. Keep the incremental state that serves the active edit
loop, but do not let a long-lived checkout grow without measurement.

- inspect the target directory by profile and artifact class before choosing a
  policy; a large `incremental` directory is not a reason to disable
  incremental compilation;
- do not run pruning automatically from `check`, `test`, `build`, or the
  complete gate;
- when a maintained checkout has materially stale artifacts, use a pinned tool
  such as `cargo-sweep` and expose separate `cache-prune-dry` and
  `cache-prune` recipes;
- choose the capacity per repository from a dry run. There is no machine-wide
  target-size default; native dependencies and the number of supported target
  graphs materially change the useful cache size;
- after pruning, remeasure the no-op, representative source-edit, strict lint,
  and fast-test paths. Reclaimed disk is not a win if common work immediately
  recompiles most of the graph;
- independent Cargo workspaces in one checkout may share a repository-local
  target directory when an A/B test shows useful artifact or disk reuse. Never
  turn that into a machine-global target directory.

Treat pruning as explicit, low-frequency maintenance. `cargo clean` remains the
complete reset when that is actually intended.

## Benchmark protocol

Start with `cargo build --timings` in a fresh isolated target directory. Use
the report's critical path and unit durations to distinguish local-crate work,
native build scripts, dependency features, and linking before choosing an
optimization. Confirm suspicious feature duplication with `cargo tree -e
features` and version duplication with `cargo tree --duplicates`; do not infer
that two timing units with the same version are two downloaded versions because
Cargo may compile one version more than once for distinct targets or feature
sets.

Record three timings before and after a build-system change:

1. **Cold**: new isolated `CARGO_TARGET_DIR`, populated dependency cache.
2. **Warm no-op**: repeat the identical command in the same target directory.
3. **Incremental edit**: make one representative, semantics-neutral source
   edit in a temporary Git worktree and repeat the command.

Run each case at least three times after one untimed warm-up. Record the median,
Rust version, command, target/features, linker, CPU concurrency, sccache state,
cache-hit counts, and whether another build was competing for the Cargo package
cache. Never delete a developer's normal `target/` directory to manufacture a
cold result.

Compare the actual inner-loop command, not only `cargo build --release`. Adopt
an optimization when the relevant median improves by roughly ten percent or
removes meaningful variance without regressing the other common cases. Keep
the raw result local unless it is needed for a durable engineering decision;
commit the resulting policy, not transient benchmark output.

## Final review checklist

- The command ran inside the pinned Nix shell from the repository root.
- Format, Clippy with `-D warnings`, dependency policy, tests, and shipped builds
  passed with locked dependencies.
- Every Cargo workspace and shipped target graph was included.
- New policy exceptions are exact, explained, and removable.
- Edition changes received semantic review beyond `cargo fix`.
- Cache and profile changes have before/after measurements.
- Live boundaries that cannot run locally are named explicitly.

## Primary references

- [Cargo profiles](https://doc.rust-lang.org/cargo/reference/profiles.html)
- [Cargo Rust-version policy](https://doc.rust-lang.org/cargo/reference/rust-version.html)
- [Rust edition migration](https://doc.rust-lang.org/stable/edition-guide/editions/transitioning-an-existing-project-to-a-new-edition.html)
- [Advanced edition migrations](https://doc.rust-lang.org/edition-guide/editions/advanced-migrations.html)
- [Clippy usage and lint groups](https://doc.rust-lang.org/stable/clippy/usage.html)
- [sccache Rust caveats](https://github.com/mozilla/sccache/blob/main/docs/Rust.md)
- [Cargo build cache](https://doc.rust-lang.org/cargo/reference/build-cache.html)
- [cargo-sweep](https://github.com/holmgr/cargo-sweep)
- [cargo-nextest](https://nexte.st/)
