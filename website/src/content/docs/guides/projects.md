---
title: Register projects
description: Add, update, and remove repositories in a Lasso workspace.
---

## Add

```bash
lasso project add --project=app --repo-url=git@github.com:you/app.git
lasso setup --only=app
```

Registry entry (`project-defs/app/project.toml`) contains only:

```toml
kind = "external"
repo = "git@github.com:you/app.git"
default_branch = "main"
```

Kinds:

- `external` — independent Git repo, ordinary clone at `projects/<name>/`
- `subdir` — content committed under `projects/<name>/` in the workspace

Skill: **`project-add`**.

## Inspect

```bash
lasso project list --format=json
lasso project path app
```

## Update / remove

```bash
lasso project update --project=app
lasso project update --project=app --check
lasso project rm --project=app --yes
```

Skills: **`project-update`**, **`project-rm`**.

## Boundary

Do **not** put build commands, deploy metadata, credentials, or API catalogs in
`project.toml`. Resolve the checkout, then follow that project's `AGENTS.md`.
