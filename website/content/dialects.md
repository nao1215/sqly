---
title: SQL dialects
description: --dialect translates MySQL, PostgreSQL, and GoogleSQL to SQLite before running. What it translates, what it rejects, and what it cannot fix.
weight: 55
---

sqly runs SQLite. `--dialect` lets you write MySQL, PostgreSQL, or GoogleSQL (BigQuery / Cloud Spanner) and translates the query before running it.

```shell
sqly --dialect mysql      --sql "SELECT \`user_name\`, IF(\`identifier\` = 1, 'first', 'other') FROM \`user\`" user.csv
sqly --dialect postgresql --sql "SELECT user_name, identifier::text FROM \"user\" WHERE user_name ILIKE 'b%'" user.csv
sqly --dialect googlesql  --sql "SELECT SAFE_DIVIDE(total, n) AS avg FROM stats" stats.csv
```

In the shell, `.dialect` shows or switches it for the rest of the session:

```text
sqly:~/data(table)$ .dialect mysql
dialect set to mysql
sqly:~/data(table)$ .dialect
current dialect: mysql (available: sqlite, mysql, postgresql, googlesql)
```

Loading a file always uses SQLite. Only the query text is translated.

## This is translation, not emulation

Your query goes through three possible fates, and it is worth knowing which one you are relying on.

| Fate | What happens |
|:--|:--|
| Translated | Rewritten, or backed by a helper function, so the answer matches the source dialect |
| Rejected | A construct with no SQLite equivalent fails with an error naming it |
| Passed through | SQLite runs it as written — and the answer may differ from the source dialect, with no error |

The third row is the one to watch. Everything below is asserted by [`e2e/atago/dialect_limits.atago.yaml`](https://github.com/nao1215/sqly/blob/main/e2e/atago/dialect_limits.atago.yaml), which runs in CI.

## What is translated

Division, casts, string and date functions, regex operators, and the aggregates SQLite lacks. A sample per dialect:

```shell
# 5/2 is 2.5 in MySQL and GoogleSQL, 2 in PostgreSQL and SQLite. Each is honored.
sqly --csv --dialect mysql      --sql "SELECT 5/2 AS x" user.csv
sqly --csv --dialect postgresql --sql "SELECT 5/2 AS x" user.csv

# MySQL: backticks, DATE_ADD, DIV, CAST, EXTRACT, DATE_FORMAT, GROUP_CONCAT ... SEPARATOR
sqly --csv --dialect mysql --sql "SELECT DATE_ADD('2026-01-31', INTERVAL 1 MONTH) AS d" user.csv

# PostgreSQL: :: casts, ILIKE, SPLIT_PART, ~ and ~*, STRING_AGG, SIMILAR TO, numeric TO_CHAR
sqly --csv --dialect postgresql --sql "SELECT 'abc' SIMILAR TO 'a%' AS m" user.csv

# GoogleSQL: SAFE_CAST, SAFE_DIVIDE, DATE_DIFF, FORMAT_DATE, COUNTIF, LOGICAL_AND
sqly --csv --dialect googlesql --sql "SELECT SAFE_DIVIDE(1, 0) AS x" user.csv
```

`LIKE` case sensitivity follows the source dialect, casts raise where the source raises, and `DATE_ADD` on a month boundary clamps the way the source clamps. The full set is in [filesql's dialect package](https://github.com/nao1215/filesql/tree/main/dialect); the behavior is pinned by the `dialect_*.atago.yaml` suites.

## What is rejected

A construct with no SQLite equivalent fails with an error that names it, rather than reaching SQLite and producing a confusing parse error:

```shell
$ sqly --dialect postgresql --sql "SELECT DISTINCT ON (g) g, v FROM t" t.csv
translate error (postgresql): dialect: syntax not supported on SQLite backend: DISTINCT ON is not supported: SELECT DISTINCT ON (g) g, v FROM t
```

| Dialect | Rejected |
|:--|:--|
| PostgreSQL | `DISTINCT ON`, `LATERAL`, array literals (`ARRAY[...]`) |
| GoogleSQL | `QUALIFY`, `SELECT * EXCEPT`, `ARRAY<T>` types |
| MySQL | `INTERVAL` units SQLite cannot express, `GROUP_CONCAT` combining `DISTINCT` with `SEPARATOR` |

In a batch script a translate error stops the run and names the statement.

## What passes through, and can differ

SQL that SQLite happens to accept is run as written. When the source dialect would have answered differently, you get SQLite's answer and no warning. These are the cases sqly knows about:

```shell
# MySQL's default collation is case-insensitive, so MySQL answers 1.
$ sqly --csv --dialect mysql --sql "SELECT 'A' = 'a' AS eq" t.csv
0

# MySQL runs with ONLY_FULL_GROUP_BY and rejects this. SQLite picks an arbitrary
# row per group, so sqly answers instead of telling you the query is ambiguous.
$ sqly --csv --dialect mysql --sql "SELECT g, v FROM t GROUP BY g" t.csv
a,10

# MySQL and GoogleSQL are 1-based and return '' for position 0.
$ sqly --csv --dialect mysql --sql "SELECT SUBSTR('abc', 0, 2) AS x" t.csv
a

# BigQuery spells this 'true'. SQLite has no boolean type, so TRUE is already
# the integer 1 before the cast runs.
$ sqly --csv --dialect googlesql --sql "SELECT CAST(TRUE AS STRING) AS x" t.csv
1
```

Collation is the structural one: it is a property of the column and the comparison, not something a query rewrite can supply, so any string comparison, `ORDER BY`, `DISTINCT`, or `GROUP BY` on text uses SQLite's byte ordering.

## When to reach for something else

`--dialect` exists so you can reuse a query you already have without rewriting it by hand. It is not a way to test what a query will do in production.

Use it for: running a MySQL or BigQuery query you have on hand against a local file, and porting a query between engines.

Do not use it for: validating that a query behaves identically in the real engine, anything that depends on collation or on the engine's strictness settings, or as a substitute for the engine's own test suite.

## Known limitations

Behavior sqly cannot currently reproduce is kept as executable specs under [`e2e/atago/known_bugs/`](https://github.com/nao1215/sqly/tree/main/e2e/atago/known_bugs). Each is written to assert what the source dialect does, so it fails today and would start passing if the obstacle went away. They are run separately from CI:

```shell
sh scripts/run_known_bugs.sh
```

If you hit a divergence that is not listed here, it is worth [an issue](https://github.com/nao1215/sqly/issues) — several of the translations above started that way.
