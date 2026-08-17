# Initialize a workspace

```bash
export LASSO_KIT_ROOT=/path/to/lasso
lasso init ~/my-fleet \
  --name my-fleet \
  --runtime=codex,claude \
  --module=lang-go \
  --module=security
```

Then install plugins into the agent runtime from the **workspace** directory and
register projects with `lasso project add`.

Guided path: invoke the `lasso-init` skill.
