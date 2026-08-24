---
name: maintain-crates-io-package-page
description: Create or refresh a project-owned crates.io README and crate landing page for a Rust CLI package. Use when cargo install, binary names, cargo-binstall, MSRV, features, system dependencies, architecture, platform support, security boundaries, docs.rs, or registry metadata need clearer documentation.
---

# Maintain a crates.io package page

Use the optional [package-page template](../../assets/package-page.md) as
disposable scaffolding. Copy useful content into the crate README and adapt it
freely; Lasso imposes no ongoing format.

Lead with purpose, `cargo install CRATE --locked`, the installed binary name,
and one useful command. Add `cargo-binstall` only after verifying that the
project actually publishes compatible binary artifacts. Distinguish crate,
library, and executable names.

State MSRV, default/features behavior, system dependencies, supported targets,
source-build implications, architecture/mode summary, privilege requirements,
and security boundaries. Link docs.rs for public Rust API when relevant, but do
not let API documentation displace CLI usage.

Verify `Cargo.toml` metadata selects the intended README and includes accurate
license, description, repository, homepage, keywords, and categories. Inspect
`cargo package --list` so the README and required assets are really present.
The project owns rendering, testing, publication, and future edits.
