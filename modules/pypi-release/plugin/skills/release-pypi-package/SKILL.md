---
name: release-pypi-package
description: Release or configure a Python distribution on PyPI with native Python, uv, PyPA, and GitHub tooling. Use for first publication, pending or existing PyPI Trusted Publishers, OIDC workflows, wheel and sdist inspection, attestations, token retirement, checksum comparison, or fresh pip/uv/pipx acceptance.
---

# Release a PyPI distribution

Keep builds, tests, versions, release notes, artifacts, workflows, and
acceptance in the owning project. The bundled helper only prepares the one-time
PyPI Trusted Publisher fields and opens PyPI's own configuration page.

## Establish the release contract

1. Read project guidance and `pyproject.toml`; identify distribution, import,
   and executable names separately.
2. Confirm the version source, supported Python versions/platforms, wheel
   matrix, sdist policy, entry points, bundled native files, and release notes.
3. Preserve the project's artifact boundary. If CI publishes locally built
   wheels, download and checksum those exact wheels instead of rebuilding.
4. Verify the version is absent from PyPI and consistent across source, tag,
   metadata, filenames, and changelog.

## Configure trusted publishing

PyPI can create a new project from a pending Trusted Publisher; no bootstrap
upload token is required. A pending publisher does not reserve the project
name. Existing projects use their project Publishing page.

Run the bundled [setup helper](../../scripts/setup-trusted-publisher) with
`--print` first. Use `--pending` only before the project exists. Show the exact
owner, repository, workflow filename, optional GitHub environment, project,
and native PyPI URL before the user authorizes the account-level change. PyPI
owns login, 2FA, form submission, and the stored relationship.

The publish job must have job-level `id-token: write`. Prefer PyPI's stable
`pypa/gh-action-pypi-publish@release/v1` interface, or retain a project-owned
native `uv publish --trusted-publishing always` flow that is already tested.
Do not store a PyPI API token, password, cookie, or Lasso credential for routine
releases. Use an environment only when its exact name and approval policy are
intentional.

## Build and publish

Use the project's pinned build frontend. Common native checks include:

```bash
uv build
python -m build
python -m twine check dist/*
```

Inspect wheel and sdist contents, metadata, entry points, license, README,
native platform tags, and archive checksums. Run the complete project gate
before publication. Setup success is not permission to publish or approve a
workflow.

If a bootstrap API token existed from an older flow, remove local configuration
and revoke the token at PyPI separately. OIDC migration is incomplete while a
long-lived publishing secret remains usable.

## Accept from PyPI

Wait for the exact version, query PyPI independently, compare each published
file digest with the release artifact when byte identity is promised, and test
from an empty environment:

```bash
python -m pip index versions DISTRIBUTION
uvx --from DISTRIBUTION==VERSION COMMAND --version
```

Also test `pip`, `uv tool install`, or `pipx` when the project documents them.
Exercise the installed CLI's shortest useful behavior. Report workflow success,
PyPI visibility, digest equality, clean installation, and execution separately.
