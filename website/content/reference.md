---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version`; `sqly help` and `sqly version` are read as input paths. The groups below are the ones `sqly --help` prints.

## Input

Positional arguments are the inputs: files, directories, and `http(s)` URLs. Each becomes a table named after the file, and all of them load as one operation (see [Multiple inputs](#multiple-inputs)). A workbook becomes one table per shown sheet, and an ACH or Fedwire file becomes its set of related tables; you pick one by name in SQL.

A URL is read only with [`--allow-remote`](#remote-inputs).

`--` ends the flags. Everything after it is an input, which is the only way to name a file whose name begins with `-`:

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

Remote input is default-deny. Without `--allow-remote`, an `http` or `https` URL is refused as a usage error, exit `2`, before any request is made:

```shell
# refused, and no HTTP request happens
sqly --sql "SELECT * FROM users" https://example.com/users.csv

# allowed
sqly --allow-remote \
  --sql "SELECT * FROM users" \
  https://example.com/users.csv
```

The rule covers positional arguments in every run mode (a query, `--sql-file`, `--script-file`, `--inspect`, the interactive shell) and `.import URL` typed at the prompt, piped in, or read from a `--script-file`. A session started with `--allow-remote` keeps the capability for later `.import` commands.

Giving the flag without a URL is fine: `sqly --allow-remote data.csv` runs, and so does `sqly --allow-remote` with no input. A wrapper can always pass it.

`--allow-remote` is an explicit network capability, not a sandbox or an SSRF defense. It decides whether sqly makes a request, not where the request goes, and it does not lift any of the limits on [the download itself](../formats/#remote-inputs). See [what it does not protect against](../formats/#what-allow-remote-is-not).

### What each option applies to

`--encoding`, `--row-mismatch`, and `--include-hidden-sheets` apply to every input of the run they can affect: file arguments, the files inside a directory, a URL, and the `--stdin-format` dataset. There is one encoding and one policy per run, with no per-file syntax.

| Flag | Applies to | Does not apply to |
|:--|:--|:--|
| `--encoding` | csv, tsv, ltsv, json, jsonl | Excel and Parquet (they carry their own encoding), ACH and Fedwire (defined as ASCII), and the `--sql-file` script, which is always read as UTF-8 |
| `--row-mismatch` | csv, tsv | every other format: none of them has a header row a later row can disagree with |
| `--include-hidden-sheets` | xlsx | every other format: none of them has sheets |

A Unicode BOM wins over `--encoding`.

Giving an option that cannot apply to any input is an error. `--encoding shift-jis` with no text input, or `--row-mismatch skip data.json`, stops the run before anything is imported. A directory or a URL counts as applicable, because its contents are not known until it is read. In a mixed run, such as one CSV and one Parquet, the option applies where it can and the run proceeds. `--stdin-table` is rejected without `--stdin-format` for the same reason.

The one exception is `--include-hidden-sheets` in a shell started with no inputs, because a later `.import` can name a workbook. A batch run whose known inputs include no workbook still rejects it.

### Multiple inputs

Several inputs are one import: every file loads or none of them does. If any input is unreadable or malformed, the tables already read are rolled back, the remaining inputs are never opened, and the run exits `3` having changed nothing. This holds for file arguments, a directory argument, `.import` inside a session, and a mix of files and URLs. A download that succeeded before the failure is rolled back too, and its temporary file is removed.

Two inputs that would create the same table are refused before anything is loaded, naming both sources and the table. Files in different directories are different inputs even with the same base name; a file named twice, or named alongside the directory holding it, is read once.

Explicit arguments are processed in the order given, and the files of a directory in a fixed order that is the same on every platform, so a failure names the same input on every run.

### Excel sheets

sqly imports only the sheets a workbook shows, since hidden sheets usually hold working-out such as lookup tables or old drafts. An import that skips sheets prints a count on stderr:

```text
Skipped 2 hidden sheets in book.xlsx; start sqly with --include-hidden-sheets to import them.
```

The notice does not name the sheets; [`--inspect`](#inspection) does.

`--include-hidden-sheets` imports them too. It is a session setting, so a shell started with it applies it to every later `.import`.

Excel has *hidden* sheets, which a reader can unhide from the sheet tabs, and *very hidden* ones, which only the VBA editor can. The library sqly reads workbooks with reports one flag for both, so neither kind is imported by default and both are imported with the flag.

There is no way to select individual sheets on the command line. Import the workbook and pick the table in SQL.

## Query

| Flag | Does |
|:--|:--|
| `-s`, `--sql SQL` | run one statement, then exit |
| `-f`, `--sql-file FILE` | run every SQL statement in a file, then exit |
| `--script-file FILE` | run a sqly script from a file: SQL and dot-commands, then exit |
| `--dialect NAME` | write the query in `sqlite` (default), `mysql`, `postgresql`, or `googlesql` and have it translated |

`--sql`, `--sql-file`, and `--script-file` each name the work to run, so any two together are rejected. Without one, sqly opens the interactive shell on a terminal and reads a script from stdin when it is piped. `--stdin-format` uses stdin for data, so it needs one of the three, or `--inspect`.

### SQL file or script file

A `--sql-file` holds SQL. A `--script-file` holds what the shell reads, SQL statements and dot-commands, the same text a pipe would carry.

| | `--sql-file` | `--script-file` | piped stdin |
|:--|:--|:--|:--|
| SQL statements | yes | yes | yes |
| Dot-commands | rejected by name and line | yes | yes |
| `.save`, `.import` | rejected | yes | yes |
| stdin free for `--stdin-format` | yes | yes | no |
| `--output` | yes, for one result set | rejected; use `.dump` | no |
| Several result sets | `table`, `vertical`, `markdown` only | same | same |
| On a failing statement | stops there, non-zero | stops there, non-zero | stops there, non-zero |

The flags are separate because other tools read `.sql` files too, and one that ran `.save` would have side effects nobody expects from SQL. A script that works piped in works unchanged from a file, so it can live in version control.

```shell
sqly --script-file monthly.sqly sales.csv
```

```text
# monthly.sqly
UPDATE sales SET region = 'APAC' WHERE region = 'ASIA';
.save --in-place
```

An empty `--script-file` is rejected. A script holding only dot-commands is fine.

### What a `--sql-file` may contain

A dot-command in a `--sql-file` is rejected by name and line before any statement runs:

```text
$ sqly --sql-file save.sql user.csv
save.sql runs SQL only, but line 2 is the helper command ".save"; run it with --script-file, or pipe it to sqly
```

To mix both, use `--script-file` or pipe the same text in:

```shell
printf "UPDATE user SET name = 'x' WHERE id = 1;\n.save --in-place\n" | sqly user.csv
```

### What sqly does with standard input

Whether stdin is a script, a dataset, or unused depends only on the flags and on what stdin is attached to, never on its contents. sqly does not read stdin to find out, so a slow pipe or a FIFO with no writer cannot hang it.

| Flags | stdin is | sqly does |
|:--|:--|:--|
| none | a terminal | opens the interactive shell |
| none | a pipe or a redirected file | runs it as a script: SQL and dot-commands |
| `--stdin-format FORMAT` | anything | imports it as the table `stdin` |
| `--sql` / `--sql-file` / `--script-file` / `--inspect` | a terminal, `/dev/null`, or an empty file | nothing; stdin is unused |
| `--sql` / `--sql-file` / `--script-file` / `--inspect` | a pipe or a non-empty file | nothing, and says so on stderr |

The last row catches `cat data.csv | sqly --sql "..." other.csv`, which looks like it feeds `data.csv` in but answers from `other.csv` alone:

```text
$ cat user.csv | sqly --sql "SELECT COUNT(*) AS n FROM user" user.csv
warning: standard input was not read; --sql works from the statement on the command line. To query it, add --stdin-format FORMAT.
+---+
| n |
+---+
| 3 |
+---+
```

It is a warning, not a failure, because telling an empty pipe from a full one would mean reading it, which hangs on a FIFO nobody writes to. The exit code is unchanged, so a wrapper that hands every child an empty pipe keeps working.

Naming standard input as an input file works without a warning:

```shell
cat data.csv | sqly --sql "SELECT COUNT(*) FROM stdin" /dev/stdin
```

### How many results a run may produce

`--sql` runs exactly one statement, and a second one is rejected. The split understands quotes, comments, and triggers, so a semicolon inside `'a;b'` or a `-- comment;` does not count.

`--sql-file`, `--script-file`, and a piped script run every statement. How many may return rows depends on whether the format can mark where one result ends:

| Format | Several results |
|:--|:--|
| `table`, `vertical`, `markdown` | allowed; results are printed in statement order, separated by a blank line |
| `csv`, `tsv`, `ltsv` | rejected: a second header row in the middle of the body would parse as data |
| `json` | rejected: two arrays back to back are not a JSON document |
| `jsonl` | rejected: the format has no way to mark a result boundary |
| `excel`, `parquet` | rejected: they need `--output`, which writes one file |

A rejected run writes nothing to stdout and exits non-zero; it never prints the first result and then stops. A script returning no rows (only DDL and DML) is fine in every format.

`--output` writes one file, so it needs exactly one result in any format. A script producing none, or more than one, is rejected and no file is written.

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

`excel` and `parquet` are binary with no on-screen form, so a `--sql` or `--sql-file` run that selects one without `--output` is rejected.

### What a format cannot hold

A format that cannot carry a value refuses the export. Every refusal names the column, exits `4`, and writes nothing.

| Value | csv, tsv, ltsv | json, jsonl | excel | parquet |
|:--|:--|:--|:--|:--|
| a BLOB, or any bytes that are not valid UTF-8 | refused | a string: the bytes when they are valid UTF-8, base64 otherwise | refused | the bytes are kept, typed as text on re-import |
| Infinity, -Infinity | `Infinity`, `-Infinity` | the strings `"Infinity"`, `"-Infinity"` | the same words, as text | kept as a double |
| NaN | `NaN` | the string `"NaN"` | `NaN` | NULL, which is what SQLite has for it |
| a tab, a newline or a carriage return inside a value | csv keeps it, quoted; tsv and ltsv refuse | kept | kept | kept |
| a result with no rows | csv and tsv write a header; ltsv refuses | `[]` | written | refused |
| a value longer than 32,767 characters | kept | kept | refused, because a cell holds no more | kept |

JSON quotes `Infinity`, `-Infinity`, and `NaN` the way PostgreSQL's `row_to_json` does. Parquet keeps the infinities as doubles, so one written there re-imports as a REAL, and stores NaN as NULL.

The text formats and Excel refuse a BLOB, because the bytes are usually not text and sqly's own import refuses input that is not UTF-8. `--output-format json` writes a BLOB as text when its bytes are valid UTF-8 and as base64 otherwise.

### Where the destination's extension leaves off

`--output` and the extension of its path decide the format together, and the file written is not always the path typed:

| Destination | Written to | Format |
|:--|:--|:--|
| `--output oo` | `oo.csv` | csv |
| `--output o.weird` | `o.weird` | csv |
| `--output rep --output-format json` | `rep.json` | json |
| `--output w.weird --output-format json` | `w.weird` | json |

- A known format extension picks the format. Contradicting it with `--output-format` is a usage error, exit `2`, decided before anything is read.
- An unknown extension is kept as typed, and the format is whatever `--output-format` says (`csv` by default).
- No extension gets the format's own appended.

The path actually written is always on stderr:

```text
Output sql result to report.csv (output mode=csv)
```

`.dump TABLE FILE` follows the same rules.

### Where a statement's status line goes

A statement that returns no rows prints a count instead of a result:

| `--output-format` | `affected is N row(s)` |
|:--|:--|
| `table`, `vertical`, `markdown` | stdout |
| `csv`, `tsv`, `ltsv`, `json`, `jsonl` | stderr |

Formats a program parses keep stdout for data only. The session settings `.mode`, `.dialect`, and `.row-mismatch` answer on stderr in every format.

### What `--output` guarantees

The result goes to a temporary file beside the destination and is moved into place only when complete. A failed query, a wrong result count, an unrepresentable value, or a failed write leaves an existing destination exactly as it was.

A destination that is a directory, ends in a path separator, or has no parent directory is rejected before any input is read, as is an `--output-format` that contradicts the extension.

A destination that resolves to one of the input files is rejected too, symlinks included; overwriting a source is `.save --in-place`'s job. This check needs the imported tables, so it runs after the inputs are read. It exits `4` either way.

An existing destination is overwritten without asking. Its permissions are kept, and a new file is created with `0600`.

A destination the filesystem will not open for writing is refused at exit `4`, naming the file and its mode. Without this check, a read-only file would be replaced by the rename and its mode copied onto the new content. `--output`, `.dump`, and `.save --in-place` all refuse it, and a `.save` covering several files refuses before touching any of them.

A `--output` destination that is a symlink is written through: the target receives the result and the link stays a link.

`.save --in-place` follows a symlinked source only when asked. By default it refuses and names both the link and its target:

```shell
sqly link.csv <<'EOF'
UPDATE link SET n = 2;
.save --in-place
EOF
# cannot save session:
#   - link: link.csv is a symlink to /srv/shared/real.csv; an in-place save would overwrite that file, which you did not name. Add --follow-symlinks to do it anyway, or save to a directory with .save DIR
```

An in-place save through a link overwrites a path the user never typed, which can sit outside the working directory and be shared with other programs. `.save --in-place --follow-symlinks` does it and prints the resolved path:

```text
following the symlink link.csv to /srv/shared/real.csv
Saved link to link.csv
```

The option applies to `--in-place` only. `.save DIR` never touches a source, so it accepts a symlinked source and rejects `--follow-symlinks`.

## Inspection

| Flag | Does |
|:--|:--|
| `--inspect` | print schema, row counts, and source metadata as JSON, then exit |
| `--inspect-sample N` | sample rows per table in `--inspect` (default `0`, which is schema only) |

`--inspect` is schema-only by default. Row data appears only when `--inspect-sample N` asks for it, at most `N` rows per table:

```shell
# schema, row counts, sources — and no row data
sqly --inspect data.csv

# the same, plus at most one row per table
sqly --inspect --inspect-sample 1 data.csv
```

An agent, a wrapper, or a CI job handed an unread file can describe it without printing its contents.

`--inspect` is rejected together with `--sql`, `--sql-file`, `--output`, `--output-format`, and `--dialect`, since it runs no query. A flag left at its default is not rejected. `--inspect-sample` without `--inspect`, or with a negative count, exits `2` before anything is read.

The report is one JSON document on stdout, ready for `jq`. Progress and warnings go to stderr, and a clean run leaves stderr empty. A failed run writes nothing to stdout.

The same binary with the same inputs and options produces the same bytes:

- top-level fields are in a fixed order: `schema_version`, `sqly_version`, `tables`, then `excel_sheets` when there is one.
- `tables` is sorted by table name, and is always an array.
- `columns` is in definition order, the file's column order.
- `sample_rows` is the first rows of the table in file order, capped at `--inspect-sample`. It is always an array: `[]` with no sample requested, never absent or `null`.

`sqly_version` changes between releases, so the bytes do too; `schema_version` promises the shape.

Each table carries its name, its source (a path, a URL, or `stdin` for a piped dataset), its row count, its columns, and its sample.

### Inspect JSON schema

Every report opens with two fields:

| Field | Type | Says |
|:--|:--|:--|
| `schema_version` | JSON number | how to read this document. It is `1`. |
| `sqly_version` | JSON string | which binary wrote it; the same string `sqly --version` prints. Never empty. |

```json
{
  "schema_version": 1,
  "sqly_version": "v1.0.0",
  "tables": []
}
```

Branch on `schema_version` and report `sqly_version`. Two releases can write the same `schema_version` with different `sqly_version` values.

The formal contract is a JSON Schema (Draft 2020-12) published at [`/sqly/schema/inspect-v1.schema.json`](../schema/inspect-v1.schema.json). It is the only copy, and sqly's tests validate real `--inspect` output against it.

#### Compatibility policy for schema version 1

A v1 consumer must ignore fields it does not know, so that the additions below do not break it.

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

A run whose inputs include an Excel workbook also gets an `excel_sheets` array listing every sheet of each workbook and which became a table. It is the only place a hidden sheet is named.

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

`visible` is what the workbook says and `imported` is what this run did, so `--include-hidden-sheets` makes `imported` true while `visible` stays false. `table` is absent for a sheet that was not imported. The whole array is absent when no workbook is among the inputs.

`source` is the same string as in the `tables` entries from that workbook: an absolute path or the downloaded URL. `sqly --inspect book.xlsx`, `sqly --inspect ./book.xlsx`, and `sqly --inspect "$PWD/book.xlsx"` all report the same path, so the two arrays can be joined on it. Sources are sorted by that value, and each workbook's sheets keep the workbook's order.

Values use the same JSON encoding as `--output-format json`:

| Value | In the JSON |
|:--|:--|
| INTEGER, REAL | a JSON number; a 64-bit integer is emitted in full, and a consumer that parses JSON numbers as doubles will lose digits past 2^53 |
| TEXT | a JSON string; `"123"` stays a string |
| NULL | `null`, which is what distinguishes it from `""` |
| BLOB | a JSON string: the bytes when they are valid UTF-8, base64 otherwise (see below) |
| Infinity, -Infinity, NaN | the JSON strings `"Infinity"`, `"-Infinity"`, `"NaN"`, because JSON has no way to write them |
| Bytes that are not valid UTF-8 in a TEXT column | the invalid bytes become U+FFFD. A text input carrying them is refused on import, so such a value reaches a TEXT column through a binary container (Parquet, Excel) or a SQL expression |

JSON distinguishes number, string, and null only. These three all come out as a JSON string and cannot be told apart:

- a TEXT value that happens to read `"YWJj"`,
- a BLOB holding the three bytes `abc`, which is valid UTF-8 and is written `"abc"`,
- a BLOB holding bytes that are not valid UTF-8, which is written base64 and
  carries no marker saying so.

So `--inspect` and `--output-format json` do not preserve the BLOB type. A program that must tell a blob from text can ask with `typeof(col)`:

```shell
sqly --output-format json --sql "SELECT typeof(payload) AS kind, payload FROM t" t.parquet
```

Base64 keeps invalid bytes recoverable; written as a string, each would become U+FFFD.

## Write back

Write-back is the shell command `.save`, used at the interactive prompt or in a script piped to sqly. It writes the tables a session changed back out to files.

| Command | Does |
|:--|:--|
| `.save DIR` | write every table the session changed into `DIR`, in its source format; the sources are untouched |
| `.save --in-place` | overwrite the source file of every table the session changed |

Overwriting the files you read cannot be undone, so it is a command placed after the statements that changed something, and it reads the same interactively and in a script:

```shell
printf "UPDATE user SET name = 'x' WHERE id = 1;\n.save --in-place\n" | sqly user.csv
```

Writing a query result somewhere is `--output`, which takes one result in any format sqly can write.

### What "changed" means

sqly fingerprints each table at import and compares before writing. A table the session did not modify is not rewritten even when a sibling was, and a session that changed nothing writes no file and says so on stderr. A value changed and changed back counts as unchanged.

The two destinations compare against different things:

| Command | Writes a table when |
|:--|:--|
| `.save --in-place` | the table differs from what its source file holds |
| `.save DIR` | the table differs from what it held at import |

So they can be used in either order:

```shell
printf "UPDATE u SET name='zoe';\n.save out\n.save --in-place\n" | sqly u.csv
```

Both the export and the source are written, and the same happens with the commands reversed. A second `.save --in-place` in either session reports "nothing to save".

Only `INSERT`/`UPDATE`/`DELETE` on an imported table are persisted. A script containing a schema change or a maintenance statement is rejected before its first statement runs, because `.save` could not represent the result. `CREATE TEMP TABLE` is fine.

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

`.save DIR` refuses a destination that already exists, and a directory that resolves back to a source file. Destinations are compared case-folded, so two tables that would land on the same file on macOS or Windows are rejected on every platform.

### How a multi-file save fails

A save covering several files writes every table to a scratch file beside its destination first, and moves them into place only when all are written. A failure while encoding the second file leaves the first untouched, and no scratch files or backups survive the run.

If a move fails after an earlier one landed (a rename can still fail, and Windows turns it into a copy when another handle holds the destination open), the files already replaced are restored from backups taken before the first move. If a restore also fails, both errors are reported, along with the file left holding partial content.

An in-place save keeps the source file's permissions.

Replacing several files cannot be atomic on any OS sqly runs on. sqly guarantees that nothing is replaced until everything is written, and that a later failure names the exact files affected.

## General

| Flag | Does |
|:--|:--|
| `-h`, `--help` | print the grouped option list and exit |
| `-v`, `--version` | print the sqly version and exit |

`sqly help` and `sqly version` are read as input paths, and sqly answers them with a hint:

```text
$ sqly help
sqly is flag-driven and has no subcommands; use "--help". Run "sqly --help" for usage. Helper commands like .tables and .import run inside the shell or batch stdin.
```

## Table name rules

Spaces, hyphens, and dots become `_`. Punctuation and symbols are removed. A name starting with a digit gets a `sheet_` prefix, and a name left with nothing becomes `sheet`. Letters and digits in any script are kept, so a file named in Japanese, Chinese, Korean, Cyrillic, or accented Latin keeps its name; quote it in queries. Excel sheet names follow the same rules.

| Input | Table |
|:--|:--|
| `bug-syntax-error.csv` | `bug_syntax_error` |
| `2023-data.csv` | `sheet_2023_data` |
| `data@v2.csv` | `datav2` |
| `売上.csv` | `売上` |
| `data.xlsx` sheet `Café` | `data_Café` |

A name that collides with a SQLite keyword is imported and a warning names it; quote it in queries.

A query against a missing table lists the tables the session has on stderr:

```text
hint: this session has no table "staf". Did you mean "staff"? Available tables: ident, staff. sqly derives table names from file names: https://nao1215.github.io/sqly/reference/#table-name-rules
```

Up to twenty names are listed, then `... (N total)`. A missing column gets a similar line pointing at `.describe TABLE` and `sqly --inspect FILE`, and so does a mistyped helper command. The suggestion is the nearest name by edit distance, where a dropped, doubled, mistyped, or swapped letter is one edit: up to two edits for names of five or more characters, one for shorter names, and no guess for names of two characters or fewer. These are hints: stdout stays empty and the run exits `1`.

## Exit codes

The exit code says which stage failed, so a script can tell a bad command line from a file that would not load without reading stderr.

| Code | Meaning | What to change |
|:--|:--|:--|
| `0` | success | |
| `1` | a statement ran and failed: a SQL error, a missing table, a constraint | the SQL |
| `2` | the command line or the script was not accepted: an unknown flag, two flags that contradict, a dot-command in a `--sql-file`, a dot-command missing an argument or given one it does not take, a script whose several result sets the chosen format cannot separate | the invocation |
| `3` | an input could not be read: a missing path, an unsupported format, a download that failed or hit a limit, a malformed row under `--row-mismatch error` | the input |
| `4` | a destination could not be written: a missing parent directory, a source with no writable form, a collision, a failed commit or rollback, a value or column set the chosen output format cannot represent, including when the destination is stdout | the destination |
| `130` | SIGINT stopped the run: someone pressed Ctrl-C | |
| `143` | SIGTERM stopped the run: something else asked it to stop | |

Most code-`2` failures are decided before anything is read or written. Two are found only while the script runs, after the inputs were read: result sets the chosen format cannot separate, and a `.mode` that contradicts a `.dump` destination. A `4` means the query may already have produced results.

A failure gets the same code at the top level or inside a script: a `.save` that cannot write exits `4` on line 9 of a piped script as it does alone.

A malformed dot-command is a `2`: `.describe` with no table, `.mode` naming an unknown mode, `.save` with two arguments, `.ls` with two paths. Once a command is accepted, its own failure is classified separately, so `.cd` to a missing directory exits `1`.

`130` and `143` are `128` plus the signal number, as a shell reports them. A Ctrl-C is usually a person changing their mind; a SIGTERM comes from a canceled CI job, a service manager, or `timeout`, which a wrapper may want to retry or report.

An interrupted run cancels the query and cleans up, removing the temp directories a download or a staged stdin dataset created. A second signal kills the process outright.

A timeout inside sqly is not a signal: a download that ran out of time exits `3`.

Errors go to stderr and query results to stdout.

## Environment

| Variable | Does |
|:--|:--|
| `SQLY_HISTORY_PATH` | where the shell keeps its command history: a text file, one entry per line. Defaults to `history` under the config directory. Unwritable disables history with one warning rather than failing the run |
