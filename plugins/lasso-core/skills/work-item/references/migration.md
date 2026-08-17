# Migration recipe

Create `migration.md` for compatibility-sensitive, staged, data-bearing, or
cross-project changes. Use `assets/migration.md`.

Record the current inventory, compatibility window, sequencing constraints,
rollout stages, observations, rollback boundary, and data/protocol validation.
Keep per-turn implementation steps in Codex Plan. A migration is complete only
after old-path removal criteria are met, not merely after the new path ships.
