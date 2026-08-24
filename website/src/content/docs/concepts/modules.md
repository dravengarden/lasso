---
title: Modules
description: Versioned capability packs installed into a workspace.
---

This page documents the module format in detail. For the product-level
extension model and ownership boundary, start with the
[extension model](/lasso/concepts/extensibility/).

## Model

The product ships `modules/catalog.toml`. Each entry is an installable pack:

```toml
[[module]]
id = "security"
version = "0.1.0"
kind = "plugin"          # plugin | convention | capability
description = "..."
path = "modules/security"
skills = ["security-scan"]
requires = []            # optional dependency ids
default = false          # install on lasso init?
```

## Workspace layout after install

```text
.lasso/modules/<id>/     copied payload
lasso.lock.toml          id → version pins
conventions/             merged from module conventions/
plugins/lasso-<id>/      materialized when module/plugin exists
```

Marketplaces (Codex + Claude/Grok) are regenerated to include installed plugin
modules alongside `lasso-core`.

## Module, plugin, and skill

The Lasso CLI installs and pins **modules**. A plugin module materializes one
agent-runtime **plugin** named `lasso-<module>`. That plugin may contain several
related **skills**, where each skill describes one triggered workflow and how
to use its native tools.

Use separate modules when users should be able to install or remove the
capabilities independently. Keep related workflows in one plugin instead of
creating a plugin for every command. For example, `npm-release` contains both
the npm release workflow and the npm package-page maintenance workflow.

## Authoring

```text
modules/<id>/
  module.toml
  conventions/     # optional
  plugin/          # optional skills + dual manifests
  scripts/         # optional helpers
```

Add a `[[module]]` block to the catalog, then `lasso module add <id>`.
