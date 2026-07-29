# Known dialect bugs

Reproductions of SQL-dialect defects observed from sqly's CLI surface. Every
scenario here is expected to fail today: it asserts the behavior sqly should
have once the underlying filesql translation is fixed, not the behavior it has
now.

These specs are deliberately outside the `e2e/atago/*.atago.yaml` glob that
`scripts/run_e2e.sh` uses, so CI stays green while the fixes land.

Run them with:

```sh
sh scripts/run_known_bugs.sh
sh scripts/run_known_bugs.sh --filter "month-end"
```

| File | Scope |
|------|-------|
| cross_dialect.atago.yaml | Defects that hit more than one dialect the same way |
| mysql_dialect.atago.yaml | MySQL-only |
| postgresql_dialect.atago.yaml | PostgreSQL-only |
| googlesql_dialect.atago.yaml | GoogleSQL / BigQuery-only |

Two kinds of defect appear here, and they are worth telling apart:

Silent wrong answers are the dangerous ones. The query succeeds and returns a
plausible value that the source dialect would never produce, so nothing warns
the user. Integer division, case-insensitive `LIKE`, and truncating casts are
all in this class.

Missing constructs fail loudly with `no such function` or a SQLite parse error.
They cost the user a workaround, but they never corrupt a result.

When a scenario here starts passing, move it into the matching
`e2e/atago/*.atago.yaml` suite so CI protects it from regressing.
