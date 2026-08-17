---
name: lasso-add-language
description: Add a language convention pack to the current Lasso workspace by installing the matching lang-* module. Use when the user wants Go, Rust, Nix, or other language conventions.
disable-model-invocation: true
---

# Add a language pack

Language support in Lasso is a **convention module**, not a compiler toolchain.

## Map request → module id

| User intent | Module id |
|---|---|
| Go | `lang-go` |
| Rust | `lang-rust` |
| Nix | `lang-nix` |

If the language is absent from `lasso module list`, say so and offer to document
a new module under the product `modules/` tree instead of inventing files.

## Install

Follow `lasso-add-module` with the mapped id:

```bash
lasso module add lang-go --format=json
```

Confirm that files landed under `conventions/code/<lang>/` and that
`lasso.lock.toml` pins the module.

Do not install system compilers; point the user at the project's own flake,
`rustup`, or language version manager.
