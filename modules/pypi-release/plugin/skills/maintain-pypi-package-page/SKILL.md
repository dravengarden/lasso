---
name: maintain-pypi-package-page
description: Create or refresh a project-owned PyPI long description and package landing page for a Python CLI distribution. Use when pip, uv, uvx, pipx, executable names, Python/platform support, architecture, security boundaries, metadata, or registry links need clearer documentation.
---

# Maintain a PyPI project page

Use the optional [package-page template](../../assets/package-page.md) only as a
starting point. Copy useful content into the project README or configured long
description; the project owns every word after that.

Lead with purpose, canonical install, shortest useful command, and one-shot
execution. Distinguish distribution name, Python import name, and console
script. Include only tested alternatives:

- `python -m pip install` for environment installation;
- `uv tool install` for a persistent CLI;
- `uvx --from` for one-shot execution; and
- `pipx` when the project supports it.

State supported Python versions, wheel/platform availability, source-build or
system dependency requirements, architecture/mode summary, privilege model,
network downloads, and security boundaries without turning the registry page
into the full documentation site.

Verify `pyproject.toml` metadata selects the intended README and content type.
Build the distribution and inspect how the long description renders when
practical. Keep package links and command examples current. Lasso neither owns
nor synchronizes the copied page.
