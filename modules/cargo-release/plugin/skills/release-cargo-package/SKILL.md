---
name: release-cargo-package
description: Release or configure a public Rust crate on crates.io with native Cargo and GitHub tooling. Use for first token-authenticated publication, crates.io Trusted Publisher setup, OIDC workflows, workspace package selection, .crate inspection, publish=false boundaries, token retirement, checksum comparison, or fresh cargo install acceptance.
---

# Release a crates.io package

Keep versioning, builds, tests, release notes, artifacts, workflows, and
acceptance in the owning project. The bundled helper only prepares the
one-time crates.io Trusted Publisher fields and opens native settings pages.

## Select the public package explicitly

Read the workspace guidance and manifests. Identify the public crate, installed
binaries, MSRV, features, system dependencies, supported platforms, README,
and release notes. Keep internal workspace crates `publish = false` unless the
project explicitly intends to support them as public API. Do not publish a
workspace merely because its members compile.

Verify the target version is absent and agrees across source, lockfile policy,
tag, `.crate` filename, changelog, and GitHub Release. Preserve locally built
artifact boundaries when the project requires exact byte identity.

## Handle first publication separately

crates.io cannot use Trusted Publishing to create a crate. The account needs a
verified email and a narrowly scoped bootstrap API token for the first version.
Inspect and package first:

```bash
cargo package --package CRATE --locked --list
cargo publish --package CRATE --locked --dry-run
```

Use `--no-verify` only when the project documents and independently tests a
specific packaging constraint. After the first version appears, run the bundled
[setup helper](../../scripts/setup-trusted-publisher) with `--print`, review the
owner, repository, workflow, environment, and crate, then apply only after user
authorization.

The routine workflow must grant job-level `id-token: write`, obtain a
short-lived token with `rust-lang/crates-io-auth-action`, and pass that token to
native `cargo publish`. Pin actions and the Rust toolchain according to project
policy. Do not store Cargo credentials or invoke Lasso in the publish job.

After OIDC succeeds, run `cargo logout` to remove local credentials and revoke
the bootstrap token on crates.io. These are separate operations. Where crates.io
offers Trusted Publishing enforcement, enable it only after the binding has
been proven.

## Publish and accept

Run the project gate, inspect `cargo package --list` and the generated `.crate`,
then publish only the named public package. A publish timeout can occur after a
successful upload, so query registry state before retrying.

For a binary crate, use fresh Cargo homes and installation roots to avoid local
cache evidence:

```bash
cargo info CRATE@VERSION --registry crates-io
cargo install CRATE --version VERSION --locked --root TEMP_ROOT
TEMP_ROOT/bin/COMMAND --version
```

Compare the crates.io checksum with the released `.crate` when the project
promises byte identity. Verify the installed binary set, not only compilation.
Report upload, index visibility, checksum equality, fresh installation, and
execution separately.
