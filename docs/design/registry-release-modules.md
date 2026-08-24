# Package registry release modules

## Decision

Lasso ships three independent plugin modules:

| Module | Materialized plugin | Skills |
|---|---|---|
| `npm-release` | `lasso-npm-release` | npm release and npm page maintenance |
| `pypi-release` | `lasso-pypi-release` | PyPI release and PyPI page maintenance |
| `cargo-release` | `lasso-cargo-release` | crates.io release and crate-page maintenance |

The split follows the user-controlled installation boundary. A workspace may
maintain Python distributions without loading npm or Cargo release guidance.
Each plugin groups related skills because publishing and registry prose share
provider terminology, acceptance commands, and security boundaries.

## Ownership

The skill may guide an end-to-end release, but it calls the owning project's
native tools and follows project guidance. Lasso does not own package identity,
versions, changelogs, builds, artifacts, tags, GitHub Releases, workflows,
approvals, publication state, or acceptance state.

Each plugin includes one deterministic helper for the residual one-time gap:
constructing and reviewing the exact GitHub Trusted Publisher identity. npm
delegates the account change to `npm trust`; PyPI and crates.io delegate to
their native browser pages. Helpers retain no credentials or sessions.

Registry-page assets are disposable scaffolding. Once copied, the project may
rewrite or delete them and has no compatibility relationship with Lasso.

## Maintainer lessons encoded in the skills

- Treat first publication separately from routine OIDC publication. PyPI can
  use a pending publisher; npm needs an existing package; crates.io requires a
  first token-authenticated version.
- A local logout and server-side token revocation are distinct evidence.
- Trusted Publisher setup does not authorize dispatching or approving a
  release.
- Preserve the project artifact boundary and compare registry digests when the
  project promises byte identity.
- A successful workflow is not registry acceptance. Query the exact version,
  install from a fresh cache/home/root, and execute the public CLI.
- Registry pages must distinguish distribution/package/crate names from import,
  library, and executable names and include only tested installer variants.
- Lasso automates authentication setup, not release semantics or package
  manager CLIs.

## Security posture

Grant `id-token: write` only to the publishing job and bind the exact repository
and workflow filename. Add a GitHub Environment only when its approval policy is
intentional. Remove long-lived publishing credentials after the OIDC path is
proven. Protect a trusted workflow as carefully as a publishing token: OIDC
reduces credential lifetime but does not make compromised release code safe.

## Upstream references

- [npm Trusted Publishers](https://docs.npmjs.com/trusted-publishers/)
- [npm trust CLI](https://docs.npmjs.com/cli/v11/commands/npm-trust/)
- [PyPI Trusted Publishers](https://docs.pypi.org/trusted-publishers/)
- [PyPI publisher security model](https://docs.pypi.org/trusted-publishers/security-model/)
- [crates.io Trusted Publishing](https://crates.io/docs/trusted-publishing)
- [Cargo publishing](https://doc.rust-lang.org/cargo/reference/publishing.html)
