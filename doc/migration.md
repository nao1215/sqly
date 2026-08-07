# Migration guide

What to change when moving between sqly releases. Only the versions that broke
something are listed; a version not named here needs nothing done.

The [CHANGELOG](../CHANGELOG.md) records every change. This file records the ones
that require an edit to a command line, a script, or a program that reads sqly's
output.

## v1.0.0-rc6 → next

### Command history is a text file, named by `SQLY_HISTORY_PATH`

It was a SQLite database at `SQLY_HISTORY_DB_PATH`. History is an append-only
log, and a file gives the same guarantee for concurrent sessions without the lock
a database made them wait on.

```text
Before:
  SQLY_HISTORY_DB_PATH=/path/to/history.db sqly data.csv
  # default: $XDG_CONFIG_HOME/sqly/history.db, a SQLite database

From now:
  SQLY_HISTORY_PATH=/path/to/history sqly data.csv
  # default: $XDG_CONFIG_HOME/sqly/history, one entry per line

  $ cat ~/.config/sqly/history
  SELECT * FROM user
  SELECT COUNT(*) FROM user WHERE id > 1
```

**What to change:** rename the variable if you set it. `SQLY_HISTORY_DB_PATH` is
no longer read, so a run that still sets it gets the default location instead of
the one it named.

An existing `history.db` is not migrated: the new file starts empty, and the old
one is left where it is. To keep the old entries, read them out once and append
them to the new file. The escaping is the format, not decoration — an entry typed
across several lines has to arrive as one line, or it comes back as several
entries:

```shell
sqlite3 -json ~/.config/sqly/history.db 'SELECT request FROM history ORDER BY id' \
  | jq -r '.[].request | gsub("\\\\"; "\\\\") | gsub("\n"; "\\n") | gsub("\r"; "\\r")' \
  >> ~/.config/sqly/history
```

A path that cannot be written still disables history with one warning and lets
the run continue; the warning now names `SQLY_HISTORY_PATH`. sqly does not create
the directory of a path you name — only the default one under the config home —
so a path in a directory that does not exist disables history rather than
building a tree for a typo.

### `.mode excel` and `.mode parquet` are rejected

`.mode` names what the screen shows, and neither format has an on-screen form.
Selecting one printed CSV under another name, which the banner admitted.

```text
Before:
  sqly:~$ .mode parquet
  Change output mode from table to parquet (active only when executing .dump, otherwise same as csv mode)
  sqly:~$ .dump user out.parquet

From now:
  sqly:~$ .mode parquet
  invalid output mode "parquet": want table, vertical, csv, tsv, ltsv, json, jsonl,
  markdown (excel and parquet name a file, not a screen: write one with
  .dump TABLE FILE.xlsx or --output)          # exit 2

  sqly:~$ .dump user out.parquet              # the extension names the format
```

**What to change:** drop the `.mode` line from a script that pairs it with a
`.dump`. In a display mode (`table`, `vertical`) the destination's extension
picks the format, which is what those scripts were doing the long way. A `.dump`
to a path with no extension needs one now (`.dump user out.parquet`, not `.dump
user out`), since the mode is no longer there to name the format.

Extension inference applies in a display mode only: `.mode csv` followed by
`.dump user out.xlsx` is a conflict and exits `2`, as it did before.

`--output-format` is unchanged and still takes both, because there the name picks
a file format for `--output`:

```shell
sqly --output-format parquet --output q.parquet --sql "SELECT * FROM user" user.csv
```

### `.header` is removed; use `.describe`

`.header TABLE` printed a table's column names, which is the `name` column of
what `.describe TABLE` already printed. Two commands answering one question is a
choice a reader had to make for nothing, so the subset is gone.

```text
Before:
  sqly:~$ .header user
  +------------+
  |    user    |
  +------------+
  | user_name  |
  | identifier |
  +------------+

From now:
  sqly:~$ .header user
  no such sqly command: .header user     # exit 1

  sqly:~$ .describe user
  +-----+------------+---------+---------+------------+----+
  | cid |    name    |  type   | notnull | dflt_value | pk |
  +-----+------------+---------+---------+------------+----+
  |   0 | user_name  | TEXT    |       0 |            |  0 |
  |   1 | identifier | INTEGER |       0 |            |  0 |
  +-----+------------+---------+---------+------------+----+
```

**What to change:** replace `.header TABLE` with `.describe TABLE`. A script
reading the column names out of a structured mode reads the `name` key instead of
`column`:

```shell
printf '.mode jsonl\n.describe user\n' | sqly user.csv | jq -r '.name'
```

`sqly --inspect FILE` reports the same columns without a session.

## v1.0.0-rc5 → v1.0.0-rc6

### A one-column CSV or TSV result writes `""` for an empty value

The rule already applied to `--output`; stdout wrote a blank line, and a blank
line is not a record, so the row vanished when the stream was read back.

```text
Before rc6:
  sqly --output-format csv --sql "SELECT v FROM t" data.csv > out.csv
  # a row whose v is empty became a blank line
  sqly --sql "SELECT COUNT(*) FROM out" out.csv   # 2, from 3 rows

From rc6:
  # the row is written ""
  sqly --sql "SELECT COUNT(*) FROM out" out.csv   # 3
```

**What to change:** nothing, unless a consumer matched the blank line. A row of
several columns is unaffected; only a result of exactly one column changes.

### An Excel export refuses a result whose last row is empty in every column

A workbook stores cells, not rows, so such a row leaves nothing behind to say it
was there and a reader stops at the last row with a value.

```text
Before rc6:
  # 3 rows exported, 2 rows read back, exit 0

From rc6:
  # excel: the last row is empty in every column, and a workbook has no way to
  # store it: the file would read back one row short
  # exit 4, and no file is written
```

**What to change:** export to csv, tsv, json, or parquet, which carry the row, or
add a column that is not empty (`SELECT *, 1 AS n`). An empty row anywhere but
last is unaffected.

### Three refusals exit `4` or `2` where they exited `1`

`1` means a statement ran and failed. None of these ran anything.

```text
--output out.csv.bz2, out.parquet.gz, out.xlsx.gz   1 → 4
.dialect oracle, .dialect a b                       1 → 2
```

**What to change:** a wrapper that read `1` as "the SQL was wrong" now sees `4`
for a destination it cannot write and `2` for a command line it cannot accept.
`--dialect oracle` on the command line already exited `2`.

### A non-finite number is spelled `Infinity` in every format

`--output-format json` already wrote `"Infinity"`; the text formats wrote Go's
`+Inf`.

```text
Before rc6:
  sqly --output-format csv --sql "SELECT 1e400 AS v"   # +Inf
From rc6:
  # Infinity
```

**What to change:** a consumer matching `+Inf` matches `Infinity`, `-Infinity`,
or `NaN` instead.

### A column's declared type matches what is stored in it

`1_000` and `0x1p4` are numbers to Go's parser and not to SQLite's numeric
affinity, and a datetime beside a number used to lose to it. Either way the
column was reported as `INTEGER` or `REAL` while its values were stored as text.

```text
Before rc6:
  # a column of "1_000" reported "type": "REAL", typeof() said text
From rc6:
  # "type": "TEXT", which is what the storage always was
```

**What to change:** nothing, unless a program depended on the wrong type. The
values themselves never changed.

## v1.0.0-rc4 → v1.0.0-rc5

### An Excel export refuses a value XLSX cannot carry

XLSX is XML, and XML 1.0 has no way to write a control character other than tab,
newline, and carriage return, nor the noncharacters `U+FFFE` and `U+FFFF`. The
writer used to substitute `U+FFFD` for them, so the export succeeded and the byte
was gone.

```text
Before rc5:
  printf 'id,v\n1,A\x01B\n' > ctl.csv
  sqly --output-format excel --output out.xlsx --sql "SELECT * FROM ctl" ctl.csv
  # exit 0, and out.xlsx holds "A\uFFFDB"

From rc5:
  # excel: value for column "v" contains the character U+0001,
  # which XLSX cannot represent; remove it or export to csv/tsv/json
  # exit 4, and out.xlsx is untouched
```

**What to change:** a pipeline that exported such data and got a file now has to
handle a failure. Strip the character in SQL — `replace(v, char(1), '')` — or
export to csv, tsv, or json, which carry it unchanged. Tab, newline, and carriage
return still export.

### A password in a remote URL is redacted in what sqly prints

`--inspect` wrote the URL as given into `tables[].source`, and every message about
a download repeated it. Both now show what `url.Redacted` gives.

```text
Before rc5:
  sqly --allow-remote --inspect "https://user:secret@host/data.csv"
  # "source": "https://user:secret@host/data.csv"

From rc5:
  # "source": "https://user:xxxxx@host/data.csv"
```

**What to change:** a program that read `source` and fetched from it keeps the URL
it passed in instead. The field is a display of where a table came from, not a
handle to re-open it. A local path is unaffected — only `http` and `https` are
rewritten, so a Windows path keeps its drive letter.

### Ctrl-C in the interactive shell

Not a change to a command line, but to what the key does. It used to end the
session with exit code `1`; it now discards the line being typed, and stops a
statement that is already running. A canceled statement rolls back and the
session carries on, so a session that ends normally afterward still exits `0`.

## v1.0.0-rc3 → v1.0.0-rc4

### `excel_sheets[].source` is an absolute path

The `--inspect` report names a workbook twice: once as the source of each table
it produced, and once as the source of each sheet it held. The second one was the
path as it was typed, so the two did not match.

```text
Before rc4:
  sqly --inspect book.xlsx
  # tables[0].source       = "/abs/path/book.xlsx"
  # excel_sheets[0].source = "book.xlsx"

From rc4:
  # both are "/abs/path/book.xlsx"
```

**What to change:** a consumer that keyed `excel_sheets` on the relative path
keys on the absolute one, or joins the two arrays on `source` now that they
agree. A remote workbook still reports the URL it was downloaded from.

`schema_version` stays `1`. The field's type and meaning did not change; the
implementation caught up with what the schema described.

### Four failures moved off exit code `1`

`1` means a statement ran and failed, so a wrapper reading it goes back to the
SQL. Four failures that had nothing to do with SQL were reported that way and now
carry the code that says what to fix.

| Failure | Before | Now |
|:--|:--|:--|
| `--output` or `.dump` names one of the run's source files | `1` | `4` |
| the output format cannot represent a value or a column set, to a file or to stdout | `1` | `4` |
| `--output-format` contradicts the destination's extension | `1` | `2` |
| `--inspect` with no input files | `1` | `2` |

**What to change:** a wrapper that branched on `1` for any of these branches on
the new code. Nothing about the messages changed, so one matching on stderr text
needs nothing.

The format conflict also moved earlier. It is decided from the command line
before any input is read, so a run that names both a contradiction and a missing
file now reports the contradiction:

```text
Before rc4:
  sqly --output-format csv --output m.json --sql "SELECT 1" nosuch.csv
  # exit 3, "Import failed" on stderr

From rc4:
  sqly --output-format csv --output m.json --sql "SELECT 1" nosuch.csv
  # exit 2, nothing imported
```

The same contradiction inside a script — a `.mode` that disagrees with a `.dump`
destination — stays a runtime check, because the mode is session state. It exits
`2` at the statement that hits it.

### `.dialect` writes to stderr

`dialect set to mysql`, and the `current dialect: ...` line a bare `.dialect`
prints, were written to stdout. A script that named its dialect therefore put a
control line in the middle of the data:

```text
Before rc4:
  sqly --output-format json --script-file d.sqly t.csv | jq .
  # jq: parse error: Invalid numeric literal at line 1, column 8

From rc4:
  # stdout holds the JSON document, and both lines are on stderr
```

**What to change:** nothing, if you were piping stdout into a parser — that case
was broken and now works. A script that captured stdout to read the confirmation
reads stderr instead.

**Check if:** you have a fixture comparing sqly's stdout for a run that switches
dialect. The line has moved out of it.

### `.mode` and `.row-mismatch` with no argument report instead of failing

The three session settings — `.dialect`, `.mode`, `.row-mismatch` — answered an
argument-less call three different ways. `.dialect` reported the current value
and succeeded; the other two failed the run.

```text
Before rc4:
  printf '.mode\n' | sqly t.csv
  # exit 1, ".mode requires a mode name" and the mode list on stderr

From rc4:
  printf '.mode\n' | sqly t.csv
  # exit 0, "current output mode: table (available: ...)" on stderr
```

**What to change:** nothing in a script that passes a value. A mode or policy
name that does not exist is still rejected, so a typo still fails the run.

**Check if:** you have a script that relied on a bare `.mode` or `.row-mismatch`
stopping it. It now continues, under the setting that was already in effect.

### `--inspect` refuses an explicit `--dialect`

`--dialect` translates the SQL a run executes, and `--inspect` executes none, so
the flag was accepted and discarded without a word.

```text
Before rc4:
  sqly --dialect mysql --inspect t.csv
  # exit 0, the report, and the dialect ignored

From rc4:
  sqly --dialect mysql --inspect t.csv
  # exit 2, "--inspect cannot be combined with --dialect mysql"
```

**What to change:** drop the `--dialect` from an `--inspect` run. It never did
anything there. A flag left at its default is not rejected, so plain
`sqly --inspect data.csv` is unaffected.

### A `--sql` failure is no longer framed as a batch

`--sql` takes one statement, and its failure was wrapped in the wording a
multi-statement script uses:

```text
Before rc4:
  sqly --sql "SELECT * FROM nope" t.csv
  # batch statement 1 failed at line 1: "SELECT * FROM nope": execute query error: ...
  # hint: ...
  # batch stopped: statement failed

From rc4:
  # execute query error: ...: SELECT * FROM nope
  # hint: ...
```

**What to change:** nothing, unless a wrapper matches on `batch statement` or
`batch stopped` for a `--sql` run. A `--sql-file`, a `--script-file`, and a piped
script still print both, because with several statements the position is the only
way back to the one that failed. Exit codes are unchanged.

## v1.0.0-rc2 → v1.0.0-rc3

Two defaults changed, in the same direction and for the same reason: sqly is run
by wrappers, CI jobs, and LLM agents as often as by people, and both defaults
did something a caller had not asked for. Neither is a silent change — the old
command now fails loudly rather than behaving differently.

### Remote input needs `--allow-remote`

sqly no longer downloads an `http(s)` URL unless the session was given the
capability.

```text
Remote input before rc3:
  sqly --sql "..." https://example.com/data.csv

Remote input from rc3:
  sqly --allow-remote --sql "..." https://example.com/data.csv
```

**What to change:** add `--allow-remote` to any invocation that names a URL, and
to any session whose `.import` names one.

**What happens if you do not:** the run exits `2` with a message naming the flag,
before any HTTP request is made. Nothing is imported, no temporary file is
created, and stdout stays empty. A command that used to work now fails; it does
not silently do something else.

This covers every way a URL reaches sqly:

| Where the URL is | Needs the flag |
|:--|:--|
| a positional argument to any run mode | yes |
| a positional argument with `--inspect`, `--sql-file`, or `--script-file` | yes |
| `.import URL` at the interactive prompt | yes, granted when the session started |
| `.import URL` in a piped script or a `--script-file` | yes |

A session started with `--allow-remote` keeps the capability for the `.import`
commands typed later in it. Passing the flag on a run that has no URL is not an
error, so a wrapper can pass it unconditionally.

`--allow-remote` is an explicit network capability, **not a sandbox and not an
SSRF defense**. It decides whether sqly makes a request, not where the request
may go: with the flag given, sqly fetches `localhost`, a private range, or a
cloud metadata endpoint exactly as it fetches anything else. What it gives you is
a way for a wrapper that fixes sqly's argument list to turn sqly's downloading
off. It is no defense against a caller that can add flags itself. See
[Remote inputs](https://nao1215.github.io/sqly/formats/#remote-inputs).

The flag does not lift any existing limit: `http` and `https` only, five
redirects, the redirect-scheme check, the header and transfer timeouts, and the
2 GiB response body cap all still apply.

### `--inspect` is schema-only by default

```text
Inspect before rc3:
  --inspect included five sample rows by default.

Inspect from rc3:
  --inspect is schema-only by default.
  Use --inspect-sample N to include row data.
```

**What to change:** a caller that relied on `sample_rows` being populated must
pass `--inspect-sample N` with the number of rows it wants.

**What happens if you do not:** `sample_rows` is `[]`. It is still present, still
an array, and never `null`, so a consumer that iterates it sees zero rows rather
than failing on a missing key.

Everything else in the report is unchanged: table names, sources, row counts,
columns, and the `excel_sheets` array all mean what they meant in rc2.

A negative `--inspect-sample` is now rejected while the command line is parsed,
so it exits `2` before anything is read instead of exiting `1` after the import.
`--inspect-sample` without `--inspect` already exited `2` and still does.

### `--inspect` gained two top-level fields

```json
{
  "schema_version": 1,
  "sqly_version": "v1.0.0-rc3",
  "tables": []
}
```

These are additive: a consumer reading only `tables` is unaffected.

- `schema_version` is a JSON **number**. Branch on it. It is `1`, and it changes
  only for a change a consumer cannot absorb by ignoring unknown fields.
- `sqly_version` is a JSON **string**, the same one `sqly --version` prints.
  Report it; do not branch on it.

The two are easy to confuse and are not interchangeable. The compatibility
policy, and the formal
[JSON Schema](https://nao1215.github.io/sqly/schema/inspect-v1.schema.json), are
on the [reference page](https://nao1215.github.io/sqly/reference/#inspect-json-schema).

Because `sqly_version` moves between releases, the report's bytes are not stable
across versions. What is stable is that the same binary, given the same inputs
and the same options, produces the same bytes.

### A non-SQLite `--dialect` says so on stderr

Choosing `mysql`, `postgresql`, or `googlesql` now prints one line to stderr:

```text
Warning: PostgreSQL syntax is translated to SQLite; execution uses SQLite semantics, not PostgreSQL semantics.
```

**What to change:** most likely nothing. It goes to stderr, once per session, so
stdout — JSON, NDJSON, CSV, TSV, and the `--inspect` report — is untouched and
exit codes are unchanged.

**Check if:** you have a wrapper that treats any stderr output as a failure, or
that compares stderr against a fixture. Those need to allow the line.

It is printed at most once per session: before the first statement of a
`--dialect` run, or at the moment `.dialect` switches to a non-SQLite dialect in
the shell. Switching back to SQLite and out again does not repeat it. `sqlite`,
`--help`, `--version`, `--inspect`, and a rejected command line print nothing.

## Earlier versions

See the Migration Notes sections in the [CHANGELOG](../CHANGELOG.md).
