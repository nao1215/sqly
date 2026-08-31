---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version` — `sqly help` and `sqly version` are read as input paths.

The groups below are the ones `sqly --help` prints.

## Input

Positional arguments are the inputs: files, directories, and `http(s)` URLs. Each
becomes a table named after the file. They are loaded as one operation — see
[Multiple inputs](#multiple-inputs). A workbook becomes one table per sheet it
shows, and an ACH or Fedwire file becomes its related set of tables; you pick the
one you want by name in SQL, the same way you pick among a directory's files.

A URL is read only with [`--allow-remote`](#remote-inputs).

`--` ends the flags. Everything after it is an input, which is the only way to
name a file whose own name begins with `-`:

```shell
sqly --inspect -- -.csv
```

Without it the name is parsed as flags:

```text
$ sqly --inspect -.csv
unknown shorthand flag: '.' in -.csv
```

| Flag | Does |
|:--|:--|
| `--stdin-format FORMAT` | read stdin as a dataset instead of as SQL: `csv`, `tsv`, `ltsv`, `json`, `jsonl` |
| `--stdin-table NAME` | table name for the `--stdin-format` dataset (default `stdin`) |
| `--encoding ENCODING` | decode text inputs that have no BOM as this encoding (default `utf-8`) |
| `--row-mismatch POLICY` | a CSV/TSV row whose field count differs from the header: `error` (fail the import), `skip` (drop the row), `pad` (fill a short row, fail on a long one) |
| `--include-hidden-sheets` | import the sheets an Excel workbook hides as well as the ones it shows (default: only the shown ones) |
| `--allow-remote` | allow this session to download `http(s)` input it is given (default: a URL is refused before any request) |

### Remote inputs

**Remote input is default-deny.** Without `--allow-remote`, an `http` or `https`
URL is refused as a usage error — exit `2` — before any request is made:

```shell
# refused, and no HTTP request happens
sqly --sql "SELECT * FROM users" https://example.com/users.csv

# allowed
sqly --allow-remote \
  --sql "SELECT * FROM users" \
  https://example.com/users.csv
```

The capability covers every way a URL can reach sqly: positional arguments to
any run mode (a query, `--sql-file`, `--script-file`, `--inspect`, the
interactive shell), and `.import URL` typed at the prompt, piped in, or read from
a `--script-file`. A session started with `--allow-remote` keeps the capability
for the `.import` commands typed later.

Granting the capability without using it is fine: `sqly --allow-remote data.csv`
runs, and so does `sqly --allow-remote` with no input at all. A wrapper does not
have to know in advance whether the command line holds a URL.

`--allow-remote` is an explicit network capability, not a sandbox or an SSRF
defense. It decides *whether* sqly makes a request, not *where* the request may
go, and it does not lift any of the limits on
[the download itself](../formats/#remote-inputs). See
[what it does not protect against](../formats/#what-allow-remote-is-not).

### What each option applies to

`--encoding`, `--row-mismatch`, and `--include-hidden-sheets` apply to **every**
input of the run that they can affect — file arguments, the files inside a directory argument, a URL, and
the `--stdin-format` dataset alike. There is one encoding and one policy per run;
sqly has no per-file syntax for them.

| Flag | Applies to | Does not apply to |
|:--|:--|:--|
| `--encoding` | csv, tsv, ltsv, json, jsonl | Excel and Parquet (they carry their own encoding), ACH and Fedwire (defined as ASCII), and the `--sql-file` script, which is always read as UTF-8 |
| `--row-mismatch` | csv, tsv | every other format: none of them has a header row a later row can disagree with |
| `--include-hidden-sheets` | xlsx | every other format: none of them has sheets |

"Does not apply to" means the option has no effect on that input, not that
typing it is tolerated. A run whose inputs are *all* of the formats an option
cannot affect is rejected — `--row-mismatch skip data.json` fails rather than
exiting 0 having done nothing. A run with a CSV and a JSON accepts it and applies
it to the CSV.

A Unicode BOM wins over `--encoding`: a file that declares its own encoding is
read the way it declares itself.

**Typing an option that cannot apply is an error.** If you pass `--encoding
shift-jis` and no input is a text format, or `--row-mismatch skip` and no input is
CSV or TSV, the run stops before importing anything rather than accepting a flag
and ignoring it. A directory or a URL counts as applicable, because what it holds
is not known until it is read. In a mixed run — one CSV and one Parquet — the
option applies to the inputs it can and the run proceeds.

`--stdin-table` is rejected without `--stdin-format`, for the same reason.

`--include-hidden-sheets` is the one exception to the "reject a flag that cannot
apply" rule, and only in one case: a shell started with no inputs at all accepts
it, because the flag is the session's sheet policy and a later `.import` can name
a workbook. A batch run whose inputs are all known and none is a workbook is
still rejected.

### Multiple inputs

Several inputs are one import, not a sequence of them.
Either every file loads or none of them does: if any input is unreadable or malformed, the tables from the
inputs already read are rolled back, the inputs after it are never opened, and
the run exits `3` having changed nothing. There is no half-loaded state to clear
before trying again.

This applies to file arguments, a directory argument, `.import` inside a session,
and a mix of local files and URLs alike. A download that succeeded before a later
failure is rolled back with the rest, and its temporary file is removed.

Two inputs that would create the same table are
refused before anything is loaded, naming both sources and the table they share. Picking one would leave the
other's rows missing with nothing said about it. Files in different directories
are different inputs even when they share a base name; a file named twice, or
named alongside the directory holding it, is one input and is read once.

Explicit arguments are processed in the order given. The files inside a directory
argument are processed in a fixed order that does not vary by platform, so which
input a failure names is the same on every run and every machine.

### Excel sheets

sqly imports only the sheets a workbook shows. A hidden sheet usually holds the
spreadsheet's own working-out — intermediate calculations, lookup tables, an old
draft — and turning that into a queryable table surprises the reader of a file
they did not build.

An import that leaves sheets behind says so on stderr, as a count:

```text
Skipped 2 hidden sheets in book.xlsx; start sqly with --include-hidden-sheets to import them.
```

It is a count and not a list. The names of hidden sheets are the part of a
workbook its author chose not to present, so an ordinary query does not print
them; [`--inspect`](#inspect) names them, because a report of what a file holds
is exactly what it was asked for.

`--include-hidden-sheets` imports them too, and it is a session setting: a shell
started with it keeps that policy for every later `.import`.

Excel separates *hidden*, which a reader can undo from the sheet tabs, from *very
hidden*, which only the VBA editor can. sqly does not tell the two apart — the
library it reads workbooks with reports one flag covering both — so neither kind
is imported by default and both are imported with the flag.

There is no way to select individual sheets on the command line. Import the
workbook and pick the table you want in SQL.

## Query

| Flag | Does |
|:--|:--|
| `-s`, `--sql SQL` | run one statement, then exit |
| `-f`, `--sql-file FILE` | run every SQL statement in a file, then exit |
| `--script-file FILE` | run a sqly script from a file: SQL and dot-commands, then exit |
| `--dialect NAME` | write the query in `sqlite` (default), `mysql`, `postgresql`, or `googlesql` and have it translated |

`--sql`, `--sql-file`, and `--script-file` each name the work to run, so any two
of them together is rejected. Without one, sqly opens the interactive shell on a
terminal and reads a script from stdin when it is piped. `--stdin-format` takes
stdin for the data, so it needs one of the three, or `--inspect`, to say what to
run.

### SQL file or script file

A `--sql-file` holds SQL. A `--script-file` holds what the shell reads: SQL
statements and dot-commands alike, exactly the text a pipe would carry.

| | `--sql-file` | `--script-file` | piped stdin |
|:--|:--|:--|:--|
| SQL statements | yes | yes | yes |
| Dot-commands | rejected by name and line | yes | yes |
| `.save`, `.import` | rejected | yes | yes |
| stdin free for `--stdin-format` | yes | yes | no |
| `--output` | yes, for one result set | rejected; use `.dump` | no |
| Several result sets | `table`, `vertical`, `markdown` only | same | same |
| On a failing statement | stops there, non-zero | stops there, non-zero | stops there, non-zero |

The two flags are separate rather than one flag with a relaxed rule because they
make different promises. A `.sql` file is a thing other tools read too, and one
that silently ran `.save` would be a shell script wearing a SQL extension. The
flag that permits side effects says `script` in its name.

Everything else is the same, deliberately: a script that works piped in works
from a file unchanged, which is what makes it worth keeping in version control.

```shell
sqly --script-file monthly.sqly sales.csv
```

```text
# monthly.sqly
UPDATE sales SET region = 'APAC' WHERE region = 'ASIA';
.save --in-place
```

An empty `--script-file` is rejected rather than exiting 0 having done nothing. A
script that is only dot-commands is fine — it has no SQL statement in it and does
not need one.

### What a `--sql-file` may contain

A `--sql-file` holds SQL. A dot-command in one is rejected by name and line
before any statement runs:

```text
$ sqly --sql-file save.sql user.csv
save.sql runs SQL only, but line 2 is the helper command ".save"; run it with --script-file, or pipe it to sqly
```

The flag says SQL, and a `.sql` file that runs `.save` is a shell script wearing
a SQL extension. A script that mixes both is what stdin is for, and it is the
same text either way:

```shell
printf "UPDATE user SET name = 'x' WHERE id = 1;\n.save --in-place\n" | sqly user.csv
```

### What sqly does with standard input

Standard input is a script, a dataset, or unused, and which one it is never
depends on what it contains — only on the flags and on what stdin is attached to.
sqly never reads stdin to find out, so a pipe with a slow writer or a FIFO with
no writer at all cannot hang it.

| Flags | stdin is | sqly does |
|:--|:--|:--|
| none | a terminal | opens the interactive shell |
| none | a pipe or a redirected file | runs it as a script: SQL and dot-commands |
| `--stdin-format FORMAT` | anything | imports it as the table `stdin` |
| `--sql` / `--sql-file` / `--script-file` / `--inspect` | a terminal, `/dev/null`, or an empty file | nothing; stdin is unused |
| `--sql` / `--sql-file` / `--script-file` / `--inspect` | a pipe or a non-empty file | nothing, and says so on stderr |

The last row is the one worth knowing. `cat data.csv \| sqly --sql "..." other.csv`
looks like it feeds `data.csv` in, and it does not — the answer comes from
`other.csv` alone and looks perfectly correct. sqly warns rather than failing:
telling an empty pipe from a full one means reading it, and reading it is what
hangs on a FIFO nobody is writing to. The exit code is unchanged, so a wrapper
that hands every child an empty pipe keeps working.

Naming standard input as an input file works and is not warned about, because
nothing is being dropped:

```shell
cat data.csv | sqly --sql "SELECT COUNT(*) FROM stdin" /dev/stdin
```

### How many results a run may produce

`--sql` runs exactly one statement. Two would mean discarding a result, so it is
rejected — the split is quote-, comment-, and trigger-aware, so a semicolon inside
`'a;b'` or a `-- comment;` does not count as a second statement.

`--sql-file`, `--script-file`, and a piped script run every statement. How many of them may return
rows depends on the output format, because only some formats can say where one
result ends and the next begins:

| Format | Several results |
|:--|:--|
| `table`, `vertical`, `markdown` | allowed; results are printed in statement order, separated by a blank line |
| `csv`, `tsv`, `ltsv` | rejected: a second header row in the middle of the body would parse as data |
| `json` | rejected: two arrays back to back are not a JSON document |
| `jsonl` | rejected: the format has no way to mark a result boundary |
| `excel`, `parquet` | rejected: they need `--output`, which writes one file |

A rejected run writes nothing to stdout and exits non-zero; it never prints the
first result and then stops. A script that returns no rows at all (only DDL and
DML) is fine in every format.

`--output` writes one file, so it needs exactly one result whatever the format. A
script that produces none, or more than one, is rejected and no file is written —
rather than a file holding whichever result happened to be last.

## Output

| Flag | Does |
|:--|:--|
| `-o`, `--output FILE` | write the query result to a file instead of stdout |
| `--output-format FORMAT` | one of the formats below (default `table`) |

| Format | Result |
|:--|:--|
| `table` | ASCII table |
| `vertical` | one column per line, in a block per record; for rows too wide to read across |
| `csv` | CSV |
| `tsv` | TSV |
| `ltsv` | LTSV |
| `json` | JSON array preserving SQLite numeric, text, and NULL types |
| `jsonl` | newline-delimited JSON (`.jsonl`, also written `.ndjson`), same types |
| `markdown` | Markdown table |
| `excel` | Excel workbook; needs `--output` or `.dump` |
| `parquet` | Parquet; needs `--output` or `.dump` |

`excel` and `parquet` are binary files with no on-screen form, so a `--sql` or
`--sql-file` run that selects one without `--output` is rejected rather than
printing something else.

### What a format cannot hold

A format that cannot carry a value refuses the export rather than writing
something that will not read back. Every refusal names the column and exits `4`,
and nothing is written.

| Value | csv, tsv, ltsv | json, jsonl | excel | parquet |
|:--|:--|:--|:--|:--|
| a BLOB, or any bytes that are not valid UTF-8 | refused | base64 string | refused | the bytes are kept, typed as text on re-import |
| Infinity, -Infinity | `Infinity`, `-Infinity` | the strings `"Infinity"`, `"-Infinity"` | the same words, as text | kept as a double |
| NaN | `NaN` | the string `"NaN"` | `NaN` | NULL, which is what SQLite has for it |
| a tab or a newline inside a value | kept, quoted where the delimiter needs it; ltsv refuses | kept | kept | kept |
| a result with no rows | csv and tsv write a header; ltsv refuses | `[]` | written | refused |
| a value longer than 32,767 characters | kept | kept | refused, because a cell holds no more | kept |

The three words are how the text formats spell the floats that have no decimal
form; JSON quotes the same words, which is what PostgreSQL's `row_to_json`
writes, so a consumer that already handles one database's JSON handles sqly's.
Parquet is the only format with a number type wide enough for them, and it keeps
them as doubles: an infinity written there re-imports as a REAL.

A BLOB has one place to go. `--output-format json` base64-encodes it; the text
formats and Excel refuse it, because the bytes are usually not text at all and a
file holding them cannot be read back — sqly's own import refuses input that is
not UTF-8.

### Where the destination's extension leaves off

`--output` and the extension of the path it names decide the format together,
and the file that appears is not always the path that was typed:

| Destination | Written to | Format |
|:--|:--|:--|
| `--output oo` | `oo.csv` | csv |
| `--output o.weird` | `o.weird` | csv |
| `--output rep --output-format json` | `rep.json` | json |
| `--output w.weird --output-format json` | `w.weird` | json |

The three rules behind the table:

- A known format extension picks the format. Contradicting it with
  `--output-format` is a usage error, exit `2`, decided before anything is read.
- An unknown extension is left exactly as it was typed, and the format is
  whatever `--output-format` says (`csv` by default).
- No extension at all gets the format's own appended.

So `--output report` writes `report.csv` and reading `report` afterwards finds
nothing. The path that was actually written is always on stderr, and reading it
is more reliable than reproducing these rules:

```text
Output sql result to report.csv (output mode=csv)
```

`.dump TABLE FILE` follows the same rules.

### Where a statement's status line goes

A statement that returns no rows produces a count rather than a result, and
where that count is printed depends on the output format:

| `--output-format` | `affected is N row(s)` |
|:--|:--|
| `table`, `vertical`, `markdown` | stdout |
| `csv`, `tsv`, `ltsv`, `json`, `jsonl` | stderr |

A format a person reads carries the count with everything else they are
watching. A format a program parses keeps stdout to data alone, so a status line
there would have to be skipped by every consumer; it goes to stderr instead.

The session settings — `.mode`, `.dialect`, `.row-mismatch` — answer on stderr
in every format, for the same reason: a setting is not data.

### What `--output` guarantees

The result is written to a temporary file beside the destination and moved into
place only once it is complete. A query that fails, a result count that is wrong,
a format that cannot represent a value, or a failed write all leave an existing
destination exactly as it was — never truncated, half-written, or removed.

The checks that can run before the import do: a destination that is a directory,
ends in a path separator, or whose parent directory does not exist is rejected
before any input is read, as is an `--output-format` that contradicts the
destination's extension.

A destination that resolves to one of the input files is rejected too —
overwriting a source is `.save --in-place`'s job, not a side effect of
`--output`. Symlinks are resolved before that comparison, so an alias cannot get
around it. This one is not a pre-import check: which files became which tables is
only known once they have been imported, so the inputs are read before the
destination is refused. It exits `4` either way.

An existing destination is **overwritten**. `--output` is how you name the file you
want; it does not ask. The file's permissions are preserved when it already exists,
and a new file is created with the usual `0600`.

A destination the filesystem will not open for writing is refused instead, at
exit `4`, naming the file and its mode. A write stages a file beside the
destination and renames over it, which needs only a writable directory, so
without this check a file marked read-only was replaced and its mode copied onto
the new content. `--output`, `.dump`, and `.save --in-place` all refuse it, and a
`.save` covering several files refuses before any of them is touched.

A `--output` destination that is a symlink is written *through*: the file it names
receives the result and the link stays a link. A rename would replace the link
itself, leaving a regular file where the link was and the real file still holding
the old rows.

`.save --in-place` follows a symlinked source the same way, but only when asked.
By default it refuses and names both the link and what it resolves to:

```shell
sqly link.csv <<'EOF'
UPDATE link SET n = 2;
.save --in-place
EOF
# cannot save session:
#   - link: link.csv is a symlink to /srv/shared/real.csv; an in-place save would overwrite that file, which you did not name. Add --follow-symlinks to do it anyway, or save to a directory with .save DIR
```

The reason is not that following the link is wrong — it is the only correct way
to write through one. It is that an in-place save overwrites a path the user
never typed, which can sit outside the directory they are working in and can be
shared with something that did not expect sqly to rewrite it.

`.save --in-place --follow-symlinks` does it, and prints the resolved path to
stderr so the destination is visible:

```text
following the symlink link.csv to /srv/shared/real.csv
Saved link to link.csv
```

The option applies to `--in-place` only. `.save DIR` writes elsewhere and never
touches a source, so it is accepted with a symlinked source and rejects
`--follow-symlinks` as meaningless there.

## Inspection

| Flag | Does |
|:--|:--|
| `--inspect` | print schema, row counts, and source metadata as JSON, then exit |
| `--inspect-sample N` | sample rows per table in `--inspect` (default `0`, which is schema only) |

**`--inspect` is schema-only by default.** It describes what a file holds and
does not print what is in it. Row data arrives only when `--inspect-sample N`
asks for it, and then at most `N` rows per table:

```shell
# schema, row counts, sources — and no row data
sqly --inspect data.csv

# the same, plus at most one row per table
sqly --inspect --inspect-sample 1 data.csv
```

This is the default because `--inspect` is the command something reaches for
when it has been handed a file nobody has read yet — an agent, a wrapper, a CI
job. "Tell me what this is" must not answer with the contents.

`--inspect` prints a report instead of running a query, so it is rejected together
with `--sql`, `--sql-file`, `--output`, `--output-format`, and `--dialect`: each
of those asks for a different action, a different shape, or a translation of SQL
that `--inspect` never runs, and honoring one silently would mean ignoring the
other. A flag left at its default is not rejected, so `sqly --inspect data.csv`
is unaffected.
`--inspect-sample` without `--inspect` is rejected too, as is a negative count;
both exit `2` before anything is read.

The report is one JSON document on stdout and nothing else, so it can be piped
straight into `jq` or a program. Import progress and warnings go to stderr, and a
clean run keeps stderr empty. A run that fails writes nothing to stdout at all —
there is no partial document to parse.

The same binary, given the same inputs and the same options, produces the same
bytes:

- top-level fields are in a fixed order: `schema_version`, `sqly_version`,
  `tables`, then `excel_sheets` when there is one.
- `tables` is sorted by table name, and is always an array.
- `columns` is in definition order — the file's column order.
- `sample_rows` is the *first* rows of the table, in the order they were read
  from the file, capped at `--inspect-sample`. It is always an array: with no
  sample requested it is `[]`, never absent and never `null`.

Across sqly versions the bytes do change, because `sqly_version` changes. The
shape is what `schema_version` promises, not the bytes.

Each table carries its name, its source (a path, a URL, or `stdin` for a piped
dataset), its row count, its columns, and its sample.

### Inspect JSON schema

Every report opens with two fields that answer different questions:

| Field | Type | Says |
|:--|:--|:--|
| `schema_version` | JSON number | how to read this document. It is `1`. |
| `sqly_version` | JSON string | which binary wrote it — the same string `sqly --version` prints. Never empty. |

```json
{
  "schema_version": 1,
  "sqly_version": "v1.0.0",
  "tables": []
}
```

Branch on `schema_version`; report `sqly_version`. They are not
interchangeable: two releases can write the same `schema_version` and different
`sqly_version` values, which is exactly the case where the shape is stable and
the bytes are not.

The formal contract is a JSON Schema (Draft 2020-12), published at
[`/sqly/schema/inspect-v1.schema.json`](../schema/inspect-v1.schema.json). It is
the single canonical copy — there is no second copy in the repository or in this
page to drift from it — and sqly's own tests validate real `--inspect` output
against it.

#### Compatibility policy for schema version 1

**A v1 consumer must ignore fields it does not know.** That is what makes the
list below possible: everything in the first group can be added without moving
`schema_version`, and a consumer that fails on an unrecognized key would break
on a change this policy calls compatible.

These changes keep `schema_version` at `1`:

- adding an optional top-level field
- adding a field to an existing object
- adding a value to a set an existing field's description enumerates, as long as
  no existing value changes meaning
- any change a consumer that ignores unknown fields absorbs unchanged

These changes raise `schema_version`:

- removing a field
- renaming a field
- changing a field's type
- making a required field optional, or an optional field required
- changing whether a field can be `null`
- changing what an existing field means
- restructuring an array or an object
- making `sample_rows` optional again

A run whose inputs include an Excel workbook also gets an `excel_sheets` array,
listing every sheet each workbook holds and which of them became a table. It is
the only place a hidden sheet is named: a workbook contributes fewer tables than
it has sheets, and nothing in `tables` says what is missing.

```json
{
  "schema_version": 1,
  "sqly_version": "v1.0.0",
  "tables": [],
  "excel_sheets": [
    {
      "source": "/data/book.xlsx",
      "name": "Sales",
      "visible": true,
      "imported": true,
      "table": "book_Sales"
    },
    {
      "source": "/data/book.xlsx",
      "name": "Scratch",
      "visible": false,
      "imported": false
    }
  ]
}
```

`visible` is what the workbook says; `imported` is what this run did, so
`--include-hidden-sheets` turns `imported` true for a sheet whose `visible` stays
false. `table` is absent for a sheet that was not imported, because there is no
table to name. The array is absent entirely for a run with no workbook among its
inputs, so a consumer reading only `tables` sees what it always saw.

`source` is the same string here as in the `tables` entries that workbook
produced — an absolute file path, or the URL that was downloaded — whatever the
workbook was called on the command line. `sqly --inspect book.xlsx`,
`sqly --inspect ./book.xlsx`, and `sqly --inspect "$PWD/book.xlsx"` all report the
same path, so the two arrays can be joined on it. Sources are sorted by that
value and each workbook's sheets stay in the order the workbook stores them, so
two runs over the same inputs still produce the same bytes.

Values use the same JSON encoding `--output-format json` uses:

| Value | In the JSON |
|:--|:--|
| INTEGER, REAL | a JSON number — a 64-bit integer is emitted in full, and a consumer that parses JSON numbers as doubles will lose digits past 2^53 |
| TEXT | a JSON string; `"123"` stays a string |
| NULL | `null`, which is what distinguishes it from `""` |
| BLOB | a JSON string: the bytes when they are valid UTF-8, base64 otherwise (see below) |
| Infinity, -Infinity, NaN | the JSON strings `"Infinity"`, `"-Infinity"`, `"NaN"`, because JSON has no way to write them |
| Bytes that are not valid UTF-8 in a TEXT column | the invalid bytes become U+FFFD. A text input carrying them is refused on import, so such a value reaches a TEXT column through a binary container (Parquet, Excel) or a SQL expression |

The distinction JSON keeps is number / string / null, and that is the whole of
it. Three of the rows above land on "a JSON string", and a reader of the output
cannot tell them apart:

- a TEXT value that happens to read `"YWJj"`,
- a BLOB holding the three bytes `abc`, which is valid UTF-8 and is written `"abc"`,
- a BLOB holding bytes that are not valid UTF-8, which is written base64 and
  carries no marker saying so.

So **`--inspect` and `--output-format json` do not preserve the BLOB type**, and
a program that has to distinguish a blob from text must get the type from
elsewhere — `typeof(col)` in the query is the direct way:

```shell
sqly --output-format json --sql "SELECT typeof(payload) AS kind, payload FROM t" t.parquet
```

Base64 for the invalid-UTF-8 case is not a type tag; it is what keeps the bytes
recoverable at all. Written as a string, every invalid byte would become U+FFFD
and the value could not be decoded back.

## Write back

Write-back is a shell command, not a flag. `.save` writes the tables a session
changed back out to files — at the interactive prompt, or in a script piped to
sqly.

| Command | Does |
|:--|:--|
| `.save DIR` | write every table the session changed into `DIR`, in its source format; the sources are untouched |
| `.save --in-place` | overwrite the source file of every table the session changed |

It is a command rather than a flag on purpose. Writing over the files you are
reading is the one thing sqly does that cannot be undone, and it belongs at the
end of a session, after the statements that changed something — where the same
words describe it interactively and in a script:

```shell
printf "UPDATE user SET name = 'x' WHERE id = 1;\n.save --in-place\n" | sqly user.csv
```

Writing a *query result* somewhere is `--output`, a different job with different
rules: it takes one result, in any format sqly can write.

### What "changed" means

It is measured, not assumed. sqly fingerprints each table's contents at import and
compares before writing, so a table the session did not modify is not rewritten
even when a sibling table was, and a session that changed nothing writes no file
at all and says so on stderr. A net-zero edit — a value changed and changed back —
counts as unchanged.

The two destinations measure against different things, because they write
different files:

| Command | Writes a table when |
|:--|:--|
| `.save --in-place` | the table differs from what its **source file** holds |
| `.save DIR` | the table differs from what it held **at import** |

So the two can be used in either order, and each does its own job:

```shell
printf "UPDATE u SET name='zoe';\n.save out\n.save --in-place\n" | sqly u.csv
```

The export is written, and the source is written too — the export changed nothing
about the source, so the source still needs writing. Reversing the two commands
works the same way: the in-place save brings the source up to date, and the
export is still written, because it is a different file that does not hold the
table yet. A second `.save --in-place` in either session reports "nothing to
save", which by then is true.

Only `INSERT`/`UPDATE`/`DELETE` on an imported table are persisted. A script whose
statements include a schema change or a maintenance statement is rejected before
its first statement runs, because `.save` could not represent the result; a
`CREATE TEMP TABLE` is fine, since scratch space was never going to be written.

### What can and cannot be written

| Source | Write-back |
|:--|:--|
| csv, tsv, ltsv | written individually, preserving format and compression |
| parquet | written individually |
| ACH, Fedwire | the whole related table set is reconstructed into one file |
| json, jsonl, Excel | rejected by name: sqly reads them but cannot write them back |
| a table created by SQL | skipped: it has no source file |
| a table from a directory import | rejected: it is not a single source the session owns |
| a `--stdin-format` dataset | rejected: a piped dataset has no source file |
| an `http(s)` input | rejected: a remote file is not sqly's to modify |

`.save DIR` refuses a destination that already exists, and refuses a directory
that would resolve back to a source file. Destinations are compared case-folded,
so two tables that would land on the same file on macOS or Windows are rejected on
every platform rather than only where it bites.

### How a multi-file save fails

A save covering several files is all-or-nothing. Every table is written to a
scratch file beside its destination first; only when all of them have been written
are they moved into place. A failure while encoding the second file therefore
leaves the first one untouched, and neither the scratch files nor the backups
survive the run.

If a move fails after an earlier one landed — which a rename can still do, and
which Windows forces into a copy when another handle holds the destination open —
the destinations already replaced are restored from backups taken before the first
move. If a restore fails too, both errors are reported: the one that stopped the
save, and the file left holding content from a save that did not finish.

An in-place save keeps the source file's permissions. It writes through a
temporary file, so without this a world-readable CSV would come back owner-only.

**The limit:** replacing several files cannot be atomic on any OS sqly runs on.
What is guaranteed is that nothing is replaced until everything has been written,
and that a failure after that point is reported with the exact files affected.

## General

| Flag | Does |
|:--|:--|
| `-h`, `--help` | print the grouped option list and exit |
| `-v`, `--version` | print the sqly version and exit |

sqly has no subcommands, so these are flags and not words: `sqly help` and
`sqly version` are read as input paths. Typing either is recognized as the
mistake it is and answered with a hint rather than a "path does not exist".

## Table name rules

Spaces, hyphens, and dots become `_`. Punctuation and symbols are removed. A name starting with a digit gets a `sheet_` prefix, and a name left with nothing becomes `sheet`. Letters and digits in any script are kept, so a file named in Japanese, Chinese, Korean, Cyrillic, or accented Latin keeps its name — quote it in queries. Excel sheet names follow the same rules.

| Input | Table |
|:--|:--|
| `bug-syntax-error.csv` | `bug_syntax_error` |
| `2023-data.csv` | `sheet_2023_data` |
| `data@v2.csv` | `datav2` |
| `売上.csv` | `売上` |
| `data.xlsx` sheet `Café` | `data_Café` |

A name that collides with a SQLite keyword is imported and a warning names it; quote it in queries.

A query against a table this session does not have says so on stderr and lists
the ones it does, so a name that was derived rather than typed can be checked
without a second run:

```text
hint: this session has no table "staf". Did you mean "staff"? Available tables: ident, staff. sqly derives table names from file names: https://nao1215.github.io/sqly/reference/#table-name-rules
```

Twenty names are listed; past that the list is cut and the total follows it as
`... (N total)`. A missing column gets a line of the same kind, pointing at
`.describe TABLE` and `sqly --inspect FILE`. A mistyped helper command is
answered the same way. The name offered is the one a typo away — a letter
dropped, doubled, mistyped, or two swapped — and nothing is offered when no
name is that close. All are hints, not errors: stdout stays empty and the run
still exits `1`.

## Exit codes

A failing run says which stage failed, so a script can tell "the command line was
wrong" from "that file would not load" without reading stderr.

| Code | Meaning | What to change |
|:--|:--|:--|
| `0` | success | — |
| `1` | a statement ran and failed: a SQL error, a missing table, a constraint | the SQL |
| `2` | the command line or the script was not accepted: an unknown flag, two flags that contradict, a dot-command in a `--sql-file`, a dot-command missing an argument or given one it does not take, a script whose several result sets the chosen format cannot separate | the invocation |
| `3` | an input could not be read: a missing path, an unsupported format, a download that failed or hit a limit, a malformed row under `--row-mismatch error` | the input |
| `4` | a destination could not be written: a missing parent directory, a source with no writable form, a collision, a failed commit or rollback, a value or column set the chosen output format cannot represent — including when the destination is stdout | the destination |
| `130` | SIGINT stopped the run — someone pressed Ctrl-C | — |
| `143` | SIGTERM stopped the run — something else asked it to stop | — |

Most code-`2` failures are decided before anything is read or written. Two are
not. A script whose result sets the chosen format cannot separate is only known
once the script has run, so the inputs were read even though nothing was
printed. A script whose `.mode` contradicts a `.dump` destination is the same
case: the mode is session state, so the contradiction only exists at the
statement that hits it. A `4` means the query may already have produced results.

The class is the same whether a failure happens at the top level or inside a
script: a `.save` that cannot write exits `4` as line 9 of a piped script exactly
as it does on its own.

A dot-command written wrong is a `2`, whichever command it is. `.describe` with
no table name, `.mode` naming a mode that does not exist, `.save` with two
arguments, `.ls` with two paths — none of them ran, so none of them is a `1`.
What the command does once it is accepted is classified on its own: `.cd` to a
directory that is not there exits `1`, because that command ran and failed.

`130` and `143` are `128` plus the signal number, which is what a shell reports
for a process a signal killed. They are separate codes because the next move
differs: a Ctrl-C is a person changing their mind, while a SIGTERM is the
surrounding system — a canceled CI job, a service manager shutting down, a
`timeout` giving up — taking the run away, which is the case a wrapper may want
to retry or report.

An interrupted run cancels the query and returns through the normal cleanup, so
the temp directories a download or a staged stdin dataset created are removed
before sqly exits. A second signal skips that and kills the process outright.

A timeout inside sqly is not a signal: a download that ran out of time exits `3`,
as the input failure it is.

Errors go to stderr; query results go to stdout, so a pipeline stays clean.

## Environment

| Variable | Does |
|:--|:--|
| `SQLY_HISTORY_PATH` | where the shell keeps its command history: a text file, one entry per line. Defaults to `history` under the config directory. Unwritable disables history with one warning rather than failing the run |
