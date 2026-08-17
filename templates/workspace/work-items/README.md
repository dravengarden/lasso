# Work items

Work items preserve coordination facts that must outlive an agent task. They are
not an execution state machine: the active agent runtime owns the current task,
goal, plan, progress, reviewers, and conversation.

Create an item only when work needs a durable handoff, depends on an external
event, or requires an auditable decision. Use the `work-item` skill.

```bash
lasso work-item new --id=<id> --title=<outcome> --project=<name> --recipe=<hint>
lasso work-item list --format=json
lasso work-item validate
```
