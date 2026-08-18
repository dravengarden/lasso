---
title: Add and remove modules
description: Incrementally install or uninstall Lasso capability packs.
---

## List

```bash
# Available from the kit catalog
LASSO_KIT_ROOT=/path/to/lasso lasso module list

# Installed in the current workspace
lasso module list --installed --format=json
```

## Add

```bash
lasso module add lang-rust --format=json
lasso module add security
```

Effects:

- copy payload → `.lasso/modules/<id>/`
- pin version → `lasso.lock.toml`
- merge `conventions/` when present
- materialize `plugins/lasso-<id>/` when the module ships a plugin
- refresh Codex + Claude marketplaces

Skill: **`lasso-add-module`**. Language packs: **`lasso-add-language`**.

## Remove

```bash
lasso module remove security --yes
```

Requires `--yes`. Refuses when another installed module lists this id in
`requires`. Skill: **`lasso-remove-module`**.

## Built-in catalog (v0.1)

| Id | Kind | Notes |
|---|---|---|
| `docs-index` | capability | default on init |
| `lang-go` | convention | Go coding conventions |
| `lang-rust` | convention | Rust coding conventions |
| `lang-nix` | convention | Nix coding conventions |
| `security` | plugin | lightweight `security-scan` skill |

Authoring guide: [Modules concept](/lasso/concepts/modules/).
