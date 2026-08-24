# Cargo release module

Installs `lasso-cargo-release`, an agent-facing plugin for maintaining public
crates and Rust CLI packages with native Cargo and GitHub tooling. It includes
a thin one-time crates.io Trusted Publisher setup helper and a disposable
crate-page template.

The module stores no Cargo credentials and does not replace project-owned
build, publish, version, changelog, workflow, or acceptance commands.

