---
title: Modules
description: Versioned capability packs installed into a workspace.
---

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

## Authoring

```text
modules/<id>/
  module.toml
  conventions/     # optional
  plugin/          # optional skills + dual manifests
  scripts/         # optional helpers
```

Add a `[[module]]` block to the catalog, then `lasso module add <id>`.
