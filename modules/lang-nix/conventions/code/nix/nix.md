# Nix development environment

The product flake owns only the Lasso Go toolchain and deterministic repository
checks. Enter it with:

```bash
nix develop
just verify
```

Optional plugins and modules own independent flakes so their language runtimes
and dependencies do not expand the Lasso core. Verify a plugin through its own
flake and task runner; for example:

```bash
cd plugins/lasso-security
nix develop -c just verify
```

Add a package to the narrowest owning flake. Do not add Python, scanner, MCP,
or application dependencies to the Lasso product root merely because an
optional plugin uses them.

Nix builds only tracked files. Add new source files to Git before evaluating a
flake that references them. Keep root and plugin lockfiles independent so a
plugin update cannot silently change the harness development closure.
