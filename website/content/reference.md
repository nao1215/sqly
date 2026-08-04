---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version` — `sqly help` and `sqly version` are read as input paths.

The groups below are the ones `sqly --help` prints.

## Input

Positional arguments are the inputs: files, directories, and `http(s)` URLs. Each
becomes a table named after the file. A workbook becomes one table per sheet, and
an ACH or Fedwire file becomes its related set of tables; you pick the one you
want by name in SQL, the same way you pick among a directory's files.

| Flag | Does |
|:--|:--|
| `--stdin-format FORMAT` | read stdin as a dataset instead of as SQL: `csv`, `tsv`, `ltsv`, `json`, `jsonl` |
| `--stdin-table NAME` | table name for the `--stdin-format` dataset (default `stdin`) |
| `--encoding ENCODING` | decode text inputs that have no BOM as this encoding (default `utf-8`) |
| `--row-mismatch POLICY` | a CSV/TSV row whose field count differs from the header: `error` (fail the import), `skip` (drop the row), `pad` (fill a short row, fail on a long one) |

### What each option applies to

`--encoding` and `--row-mismatch` apply to **every** input of the run that they
can affect — file arguments, the files inside a directory argument, a URL, and
the `--stdin-format` dataset alike. There is one encoding and one policy per run;
sqly has no per-file syntax for them.

| Flag | Applies to | Does not apply to |
|:--|:--|:--|
| `--encoding` | csv, tsv, ltsv, json, jsonl | Excel and Parquet (they carry their own encoding), ACH and Fedwire (defined as ASCII), and the `--sql-file` script, which is always read as UTF-8 |
| `--row-mismatch` | csv, tsv | every other format: none of them has a header row a later row can disagree with |

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
--sql-file runs SQL only, but line 2 is the helper command ".save"; pipe the
script to sqly instead: printf '...' | sqly FILE
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

### What `--output` guarantees

The result is written to a temporary file beside the destination and moved into
place only once it is complete. A query that fails, a result count that is wrong,
a format that cannot represent a value, or a failed write all leave an existing
destination exactly as it was — never truncated, half-written, or removed.

The checks that can run before the import do: a destination that is a directory,
ends in a path separator, or whose parent directory does not exist is rejected
before any input is read. A destination that resolves to one of the input files is
rejected too — overwriting a source is `.save --in-place`'s job, not a side effect
of `--output`. Symlinks are resolved before that comparison, so an alias cannot
get around it.

An existing destination is **overwritten**. `--output` is how you name the file you
want; it does not ask. The file's permissions are preserved when it already exists,
and a new file is created with the usual `0600`.

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
#   - link: link.csv is a symlink to /srv/shared/real.csv; an in-place save would
#     overwrite that file, which you did not name. Add --follow-symlinks to do it
#     anyway, or save to a directory with .save DIR
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
| `--inspect` | print schema, row counts, and sample rows as JSON, then exit |
| `--inspect-sample N` | sample rows per table in `--inspect` (default 5; `0` for schema only) |

`--inspect` prints a report instead of running a query, so it is rejected together
with `--sql`, `--sql-file`, `--output`, and `--output-format`: each of those asks
for a different action or a different shape, and honoring one silently would mean
ignoring the other.

The report is one JSON document on stdout and nothing else, so it can be piped
straight into `jq` or a program. Import progress and warnings go to stderr, and a
clean run keeps stderr empty.

Two runs over the same inputs produce the same bytes:

- `tables` is sorted by table name.
- `columns` is in definition order — the file's column order.
- `sample_rows` is the *first* rows of the table, in the order they were read
  from the file, capped at `--inspect-sample` (default 5). `--inspect-sample 0`
  gives schema only, and `sample_rows` is an empty array rather than absent.

Each table carries its name, its source (a path, or `stdin` for a piped dataset),
its row count, its columns, and its sample. Values use the same JSON encoding
`--output-format json` uses:

| Value | In the JSON |
|:--|:--|
| INTEGER, REAL | a JSON number — a 64-bit integer is emitted in full, and a consumer that parses JSON numbers as doubles will lose digits past 2^53 |
| TEXT | a JSON string; `"123"` stays a string |
| NULL | `null`, which is what distinguishes it from `""` |
| BLOB | a JSON string: the bytes when they are valid UTF-8, base64 otherwise (see below) |
| Infinity, -Infinity, NaN | the JSON strings `"Infinity"`, `"-Infinity"`, `"NaN"`, because JSON has no way to write them |
| Bytes that are not valid UTF-8 in a TEXT column | the invalid bytes become U+FFFD, as they already did on import |

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

## Exit codes

A failing run says which stage failed, so a script can tell "the command line was
wrong" from "that file would not load" without reading stderr.

| Code | Meaning | What to change |
|:--|:--|:--|
| `0` | success | — |
| `1` | a statement ran and failed: a SQL error, a missing table, a constraint | the SQL |
| `2` | the command line or the script was not accepted: an unknown flag, two flags that contradict, a dot-command in a `--sql-file`, a format that cannot carry the results | the invocation |
| `3` | an input could not be read: a missing path, an unsupported format, a download that failed or hit a limit, a malformed row under `--row-mismatch error` | the input |
| `4` | a destination could not be written: a missing parent directory, a source with no writable form, a collision, a failed commit or rollback | the destination |
| `130` | SIGINT or SIGTERM stopped the run | — |

Most code-`2` failures are decided before anything is read or written. The
exception is a script whose result sets the chosen format cannot separate: that
is only known once the script has run, so the inputs were read even though
nothing was printed. A `4` means the query may already have produced results.

The class is the same whether a failure happens at the top level or inside a
script: a `.save` that cannot write exits `4` as line 9 of a piped script exactly
as it does on its own.

An interrupted run cancels the query and returns through the normal cleanup, so
the temp directories a download or a staged stdin dataset created are removed
before sqly exits.

Errors go to stderr; query results go to stdout, so a pipeline stays clean.

## Environment

| Variable | Does |
|:--|:--|
| `SQLY_HISTORY_DB_PATH` | where the shell keeps its command history |
