# Go + SQL → sqlc

> Forward-looking convention. As of 2026-05 **no Go code in this repo
> touches SQL** — the harness CLI is SQL-free, and every
> service that does use SQL (cowboy, theia, heimdall) is Rust on
> `sqlx`. This file fixes the rule **before** the first Go service
> needs a database, so the choice isn't made ad-hoc under deadline.

When a Go program in this monorepo needs to talk to a SQL database,
**write the queries as SQL and generate the Go with [sqlc](https://sqlc.dev)**.
No hand-rolled `database/sql` plumbing, no ORM, no runtime query builder.

## The rule

| Job | Use | NOT |
|---|---|---|
| Map SQL queries → typed Go methods | **sqlc** (`sqlc generate`) | hand-written `rows.Scan` boilerplate |
| Schema / migrations | **plain `.sql` files** (golang-migrate style) | ORM auto-migrate, `CREATE TABLE` in Go strings |
| Driver | **`github.com/jackc/pgx/v5`** (postgres) / `modernc.org/sqlite` (sqlite, cgo-free) | `lib/pq` (maintenance-mode), `mattn/go-sqlite3` (cgo) |
| Connection pool | **`pgxpool`** (postgres) | DIY pool |

Banned for new Go SQL code: GORM, ent, sqlx, squirrel, gorp, and any
"write Go, get SQL" abstraction. They trade compile-time SQL clarity
for runtime reflection and hidden queries.

## Why sqlc and not an ORM

- **The SQL is the source of truth.** You write the exact query; sqlc
  generates the struct + method around it. Reviewers see real SQL in
  the diff, not a fluent-builder chain that compiles to who-knows-what.
- **Compile-time checked against the schema.** sqlc parses the queries
  against your DDL at generate time — a typo't column or arity mismatch
  fails `sqlc generate`, not production.
- **No runtime reflection.** Generated code is plain `Scan` into typed
  structs; zero ORM overhead, predictable allocations.
- **Symmetry with the Rust side.** The Rust services use `sqlx`'s
  compile-time-checked `query!` macros for the same reason. sqlc is the
  Go-idiomatic equivalent: SQL-first, statically verified. (sqlc is
  Go-only — it does not apply to the Rust projects; don't try to point
  it at them. See [the Rust services'](../../../projects/) own conventions.)

## Layout

Per Go module that uses SQL:

```text
<module>/
  sqlc.yaml                 # sqlc config (version "2")
  db/
    migrations/             # NNNN_name.up.sql / .down.sql — schema source of truth
    queries/               # *.sql with -- name: Foo :one  annotations
    sqlc/                  # GENERATED — committed, never hand-edited
      db.go
      models.go
      *.sql.go
```

- **Generated code is committed**: PR diffs show what changed and CI doesn't
  need the sqlc toolchain. CI verifies the tree is clean
  (`sqlc generate` produces no diff), it does not regenerate.
- The generated package carries no hand edits. Wrap it from a
  repository/store package if you need behaviour around the queries.

## sqlc.yaml baseline

```yaml
version: "2"
sql:
  - engine: "postgresql"        # or "sqlite"
    schema: "db/migrations"
    queries: "db/queries"
    gen:
      go:
        package: "sqlcgen"
        out: "db/sqlc"
        sql_package: "pgx/v5"   # use the pgx driver, not database/sql
        emit_pointers_for_null_types: true
        emit_json_tags: true
```

## Workflow

```sh
sqlc generate     # regenerate db/sqlc/*.go from queries + schema
go test ./...     # generated code compiles + your tests run
```

After editing a query or the schema:

1. Edit the `.sql` (migration or query file).
2. `sqlc generate`.
3. Review the generated `db/sqlc/*.go` diff.
4. Commit the `.sql` edit AND the regen output in the same commit.

Provide sqlc to the dev shell (add to `flake.nix` `devShell`) so every
agent has `sqlc` on PATH — don't `go install` it ad-hoc. Wire a
`just gen` (or extend the existing one) to run `sqlc generate` alongside
other project-owned generators so a single command keeps the tree current.

## Escape hatch

For a truly dynamic query sqlc can't express (variadic `IN`, query
shape decided at runtime), drop to hand-written `pgx` for **that one
query** and leave a comment explaining why sqlc didn't fit. The default
stays sqlc; the exception is per-query and justified, never
module-wide.
