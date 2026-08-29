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

Both lines go to stderr, so a script that switches dialect still writes only its
rows to stdout.

Loading a file always uses SQLite. Only the query text is translated.

## This is translation, not emulation

Choosing a non-SQLite dialect says so once, on stderr:

```text
Warning: PostgreSQL syntax is translated to SQLite; execution uses SQLite semantics, not PostgreSQL semantics.
```

It is printed once per session, not once per statement, and at the moment the
choice is made: `.dialect` says it as it switches, and a `--dialect` given on
the command line says it at the first statement that runs under it. So
`--dialect mysql` with a `--script-file` of nothing but `.tables` says nothing,
because no statement runs under it, and the shell's banner is no longer preceded
by a warning about a query nobody has typed yet. Switching back to
SQLite and out again does not repeat it, and `sqlite` never triggers it. It goes
to stderr and never to stdout, so JSON, NDJSON, CSV, TSV, and the `--inspect`
report are unaffected and stay parseable. `--help`, `--version`, and a rejected
command line say nothing about a dialect they do not use, and `--inspect` runs no
user SQL: it refuses an explicit `--dialect` rather than discarding it.

The warning is the short form of what this section says: sqly rewrites the
*syntax* and then runs the result on SQLite. It does not emulate the source
database's semantics, types, collation, `NULL` behavior, or functions. A query
SQLite accepts runs as written, and its answer can differ from the source
dialect's with nothing said about it.

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
sqly --output-format csv --dialect mysql      --sql "SELECT 5/2 AS x" user.csv
sqly --output-format csv --dialect postgresql --sql "SELECT 5/2 AS x" user.csv

# MySQL: backticks, DATE_ADD, DIV, CAST, EXTRACT, DATE_FORMAT, GROUP_CONCAT ... SEPARATOR
sqly --output-format csv --dialect mysql --sql "SELECT DATE_ADD('2026-01-31', INTERVAL 1 MONTH) AS d" user.csv

# MySQL's operators that SQLite does not share: && is AND, ! is NOT, ^ is a
# bitwise exclusive or. || is a logical OR, as MySQL's default sql_mode reads it.
sqly --output-format csv --dialect mysql --sql "SELECT 1 && 0 AS a, !0 AS b, 5 ^ 3 AS c" user.csv

# PostgreSQL: :: casts, ILIKE, SPLIT_PART, ~ and ~*, STRING_AGG, SIMILAR TO, numeric TO_CHAR
sqly --output-format csv --dialect postgresql --sql "SELECT 'abc' SIMILAR TO 'a%' AS m" user.csv

# GoogleSQL: SAFE_CAST, SAFE_DIVIDE, DATE_DIFF, FORMAT_DATE, COUNTIF, LOGICAL_AND.
# The SAFE. call prefix BigQuery's own documentation uses is the same query.
sqly --output-format csv --dialect googlesql --sql "SELECT SAFE_DIVIDE(1, 0) AS x" user.csv
sqly --output-format csv --dialect googlesql --sql "SELECT SAFE.DIVIDE(1, 0) AS x" user.csv
```

`LIKE` case sensitivity follows the source dialect, casts raise where the source raises, and `DATE_ADD` on a month boundary clamps the way the source clamps. The full set is in [filesql's dialect package](https://github.com/nao1215/filesql/tree/main/dialect); the behavior is pinned by the `dialect_*.atago.yaml` suites.

## What is rejected

A construct with no SQLite equivalent fails with an error that names it, says where it was written, and where there is one says what to write instead, rather than reaching SQLite and producing a confusing parse error:

```shell
$ sqly --dialect postgresql --sql "SELECT DISTINCT ON (g) g, v FROM t" t.csv
Warning: PostgreSQL syntax is translated to SQLite; execution uses SQLite semantics, not PostgreSQL semantics.
translate error (postgresql): dialect: syntax not supported on SQLite backend: DISTINCT ON is not supported; write the first row of each group with a window function at line 1, column 1: SELECT DISTINCT ON (g) g, v FROM t
```

That output is asserted against the binary by
[`e2e/atago/v1_0_bugs.atago.yaml`](https://github.com/nao1215/sqly/blob/main/e2e/atago/v1_0_bugs.atago.yaml).

| Dialect | Rejected |
|:--|:--|
| PostgreSQL | `DISTINCT ON`, `LATERAL`, array literals (`ARRAY[...]`), set-returning functions (`generate_series`, `unnest`, ...) |
| GoogleSQL | `QUALIFY`, `SELECT * EXCEPT`, `ARRAY<T>` types, array literals and subscripts (`[1,2,3]`, `x[OFFSET(0)]`), a `SAFE.` prefix on a function with no safe form |
| MySQL | `XOR`, `INTERVAL` units SQLite cannot express, `GROUP_CONCAT` combining `DISTINCT` with `SEPARATOR` |

`XOR` is the shape of these judgments. SQLite has no operator whose precedence
sits between `OR` and `AND`, so `XOR`'s operands are whole `AND`-expressions
rather than the primaries a rewrite can pick out: translating it would
reassociate `a AND b XOR c` into something MySQL never meant. The error says
what to write instead, `(a AND NOT b) OR (NOT a AND b)`. A set-returning
function and an array are rejected for the same kind of reason — SQLite has no
form for either — and the rejection is what keeps the message about the query
you wrote rather than about a column or a table SQLite invented from it.

In a batch script a translate error stops the run and names the statement.

## What passes through, and can differ

SQL that SQLite happens to accept is run as written. When the source dialect would have answered differently, you get SQLite's answer and no warning. These are the cases sqly knows about:

```shell
# MySQL's default collation is case-insensitive, so MySQL answers 1.
$ sqly --output-format csv --dialect mysql --sql "SELECT 'A' = 'a' AS eq" t.csv
0

# MySQL runs with ONLY_FULL_GROUP_BY and rejects this. SQLite picks an arbitrary
# row per group, so sqly answers instead of telling you the query is ambiguous.
$ sqly --output-format csv --dialect mysql --sql "SELECT g, v FROM t GROUP BY g" t.csv
a,10

# BigQuery spells this 'true'. SQLite has no boolean type, so TRUE is already
# the integer 1 before the cast runs.
$ sqly --output-format csv --dialect googlesql --sql "SELECT CAST(TRUE AS STRING) AS x" t.csv
1
```

None of these is going to be fixed one dialect at a time. A rewrite that gives MySQL its answer and leaves PostgreSQL and GoogleSQL on SQLite's replaces one divergence you can look up with three that depend on which `--dialect` you passed. So a case is fixed when it can be fixed for every dialect that has an opinion about it, and documented here when it cannot. `SUBSTR` from position 0 is the one that left this list outright: MySQL's and PostgreSQL's rules were checked against their own engines, and GoogleSQL's was checked against a BigQuery once one could be run locally, so all three answer their own way now.

Each of these fails that test for its own reason:

- Collation is a property of the column and the comparison, not of the call being rewritten. Supplying MySQL's would mean attaching it to every string comparison, `ORDER BY`, `DISTINCT`, `GROUP BY`, `LIKE`, and `IN`; doing less leaves `=` case-insensitive while `GROUP BY` stays case-sensitive, which is harder to reason about than one byte ordering everywhere.
- `ONLY_FULL_GROUP_BY` is a strictness setting rather than a construct to translate, and reproducing it means refusing a query SQLite can answer. sqly answers with a row from each group instead, which is the more useful reading for a tool you point at a file to find out what is in it.
- The boolean cast is only recoverable where the expression is syntactically a boolean. `CAST(col AS STRING)` over a column of 0 and 1 is not, because SQLite has no boolean type and nothing downstream can tell that column from a plain integer one.

## A translated expression keeps the name you wrote

SQLite names an unaliased result column after the text of the expression that
produced it, so rewriting the expression would rename the column. sqly puts the
original text back, and that is what the result is labeled with — a JSON key, a
CSV header, a column in a table:

```shell
$ sqly --output-format json --dialect postgresql --sql "SELECT salary::text FROM staff" staff.csv
[{"salary::text":"120"}]

$ sqly --output-format json --dialect mysql --sql "SELECT CONCAT(name,'-',dept) FROM staff" staff.csv
[{"CONCAT(name,'-',dept)":"alice-eng"}]
```

The label is the expression as written, not the name the source database would
derive: PostgreSQL calls `salary::text` just `salary`, and MySQL and GoogleSQL
each name a bare cast their own way. One rule that shows your own syntax back is
more predictable than three that still all differ from SQLite.

Name the column when the output is going to be read by something:

```shell
$ sqly --output-format json --dialect postgresql --sql "SELECT salary::text AS salary_text FROM staff" staff.csv
[{"salary_text":"120"}]
```

Use `AS` wherever the label matters: an explicit name is a promise the query
makes, and it does not depend on how the expression is spelled or on how sqly
translates it.

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
