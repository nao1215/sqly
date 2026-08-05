# Migration guide

What to change when moving between sqly releases. Only the versions that broke
something are listed; a version not named here needs nothing done.

The [CHANGELOG](../CHANGELOG.md) records every change. This file records the ones
that require an edit to a command line, a script, or a program that reads sqly's
output.

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
