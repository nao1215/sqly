---
title: Formats
description: Every file format sqly reads and writes, how each one becomes a table, and the compression and text encodings it handles.
weight: 50
---

## What each format can do

The formats are not interchangeable. Reading is nearly universal; writing back
into the source file is not, and two of them are not even one table each. This
is the whole picture in one place — the sections below explain the entries.

| Format | Read | stdin | URL | Compressed | Tables per file | Query result | Write back | Types |
|:--|:--|:--|:--|:--|:--|:--|:--|:--|
| CSV | yes | yes | yes | yes | 1 | yes | yes | inferred |
| TSV | yes | yes | yes | yes | 1 | yes | yes | inferred |
| LTSV | yes | yes | yes | yes | 1 | yes | yes | inferred |
| JSON | yes | yes | yes | yes | 1 (`data` column) | yes | no | document kept whole |
| JSONL | yes | yes | yes | yes | 1 (`data` column) | yes | no | document kept whole |
| Parquet | yes | no | yes | yes (read only) | 1 | yes (needs `--output`) | yes, uncompressed | from the schema |
| Excel | yes | no | yes | yes | one per sheet | yes (needs `--output`) | no | inferred |
| ACH | yes | no | yes | no | 4: `_file_header`, `_batches`, `_entries`, `_addenda` | yes | yes, as a set | fixed by the spec |
| Fedwire | yes | no | yes | no | 1: `_message` | yes | yes, as a set | fixed by the spec |

Reading the columns:

- **stdin** — can be piped in with `--stdin-format`. The four that cannot
  (Parquet, Excel, ACH, Fedwire) are binary or multi-table: there is no filename
  to name the tables after.
- **Compressed** — readable through `.gz`, `.bz2`, `.xz`, `.zst`, `.z`,
  `.snappy`, `.s2`, `.lz4`. ACH and Fedwire are not. A compressed Parquet file
  reads, but cannot be written back: Parquet already compresses internally, and
  `.save` will not produce a doubly compressed file.
- **Query result** — can be produced by `--output-format`. Parquet and Excel are
  binary and need `--output` to write to; they are never printed.
- **Write back** — `.save` can rewrite the source. JSON and JSONL cannot,
  because the whole document lives in one column and sqly cannot reconstruct
  the file from it. Excel cannot, because several tables share the file. ACH and
  Fedwire are rebuilt from their complete set of tables into one file.
- **Types** — `inferred` means sqly reads the values and picks INTEGER, REAL, or
  TEXT; a value that looks numeric but must stay text (a zero-padded code) stays
  text. Parquet carries its own schema, and the financial formats have theirs
  fixed by the specification.

Compression is preserved by a write-back: a `.csv.gz` source is rewritten as
`.csv.gz`. A `.bz2` source cannot be written back at all — there is no bzip2
writer — and is refused before anything is touched.

## Read

| Format | Extensions | Becomes |
|:--|:--|:--|
| CSV | `.csv` | one table, columns from the header |
| TSV | `.tsv` | one table, columns from the header |
| LTSV | `.ltsv` | one table, columns from the labels |
| JSON | `.json` | one table with a `data` column holding each document |
| JSONL | `.jsonl` | one table with a `data` column holding each line |
| Parquet | `.parquet` | one table, columns from the schema |
| Excel | `.xlsx` | one table per sheet, named `file_sheet` |
| ACH | `.ach` | several tables: `_file_header`, `_batches`, `_entries`, `_addenda` |
| Fedwire | `.fed` | one `_message` table |

## Write

`--output-format csv`, `--output-format tsv`, `--output-format ltsv`, `--output-format json`, `--output-format jsonl`, `--output-format markdown`, `--output-format excel`, `--output-format parquet`, and the default `table`.

`--output PATH` writes to a file; its extension must agree with the chosen format. With the default `table` mode the format is inferred from the extension instead, falling back to CSV.

ACH and Fedwire tables can be exported to csv/tsv/xlsx like any other table. Writing them back into a valid `.ach`/`.fed` file is what `.save` does, not `--output`.

## Compression

CSV, TSV, LTSV, JSON, JSONL, Parquet, and Excel are read through `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4` — so `data.csv.gz` is table `data`, with nothing to declare.

Output compression comes from the destination's extension:

```shell
sqly --output-format csv --output out.csv.zst --sql "SELECT * FROM data" data.csv.gz
```

A write-back preserves each source's own compression.

## JSON and JSONL

A document is not flattened into columns; it lands whole in a `data` column, and SQLite's JSON functions do the rest:

```shell
sqly --sql "SELECT json_extract(data, '\$.name') AS name FROM sample" sample.jsonl
```

That keeps heterogeneous documents queryable — no schema is guessed, and a field missing from some rows is simply `NULL`. See the [cookbook](/cookbook/#json-and-jsonl) for nested fields, arrays, and flattening.

## Excel

Each sheet the workbook shows becomes its own table, so a workbook is queried the
way a directory is: pick the table you want. There is no flag to select a single
sheet, because selecting one is what the `FROM` clause is for. Sheets the
workbook hides are left out unless [`--include-hidden-sheets`](#hidden-sheets)
asks for them.

```shell
sqly --sql "SELECT * FROM book_Q3_actuals" book.xlsx
```

### How a sheet becomes a table name

The table name is the file's base name, an underscore, and the sheet name:
`book.xlsx` + sheet `Q3_actuals` becomes `book_Q3_actuals`. Both halves are
sanitized so the result is usable as a SQL identifier:

| In the sheet name | In the table name | Example |
|:--|:--|:--|
| Letters, digits, `_` | kept as they are, including case | `Sheet1` → `book_Sheet1` |
| Spaces, `-`, `.` | become `_` | `Q3 actuals` → `book_Q3_actuals` |
| Other punctuation (`()`, `#`, `$`, `%`, …) | dropped | `x(1)` → `book_x1` |
| A leading digit | prefixed with `sheet_` | `2024` → `book_sheet_2024` |
| Non-ASCII letters | kept, and the name then needs quoting | `売上` → `"book_売上"` |

`.tables` prints each name the way you have to type it, already quoted when
quoting is required, so the output can be pasted into a query. A sheet named
after a SQLite keyword needs no quoting once the file name is in front of it:
sheet `select` of `book.xlsx` is `book_select`, an ordinary identifier.

### What is and is not imported

| The workbook has | sqly does |
|:--|:--|
| Many shown sheets | each becomes its own table; a hundred shown sheets is a hundred tables |
| A hidden sheet | leaves it out, and says how many it left out; `--include-hidden-sheets` imports it |
| A very hidden sheet | the same: sqly does not tell the two kinds of hiding apart |
| A sheet with no cells | skips it; no empty table is created |
| A sheet with a header and no rows | imports it as a table with zero rows |
| No usable sheet at all | fails the import, saying the file produced no table |
| A sheet that cannot be read | fails the whole workbook; sqly does not import part of a file and report success |

Two workbooks may hold the same sheet name — `a.xlsx` and `b.xlsx` both with
`Sheet1` give `a_Sheet1` and `b_Sheet1`, because the file name is part of the
table name.

Two sheets of one workbook can want the same table name: `Q1 sales` and
`Q1.sales` both sanitize to `book_Q1_sales`, and `x(1)` and `x1` both become
`book_x1`. That import is refused before any sheet is read, naming both sheets
and the table they share:

```text
$ sqly --sql "SELECT 1" book.xlsx
sheets "Q1 sales" and "Q1.sales" of book.xlsx both map to table
"book_Q1_sales"; rename one of them
```

Rename one of the sheets. (Two sheets differing only in case cannot occur —
Excel compares sheet names case-insensitively itself.)

The check runs over the sheets that are actually imported. A hidden sheet that
would want the same table as a shown one is not a collision while it stays
unimported; adding `--include-hidden-sheets` brings it into the check and the
same workbook is then refused.

### Hidden sheets

sqly imports only the sheets a workbook shows. A hidden sheet usually holds the
spreadsheet's own working-out — intermediate calculations, a lookup table, a
draft nobody deleted — and turning that into a queryable table surprises whoever
opens a file they did not build.

An import that left sheets behind says how many, on stderr:

```text
Skipped 2 hidden sheets in book.xlsx; start sqly with --include-hidden-sheets to import them.
```

That is a count, not a list: the names of hidden sheets are the part of a
workbook its author chose not to present. `--inspect` names them, because a
report of what a file holds is exactly what it was asked for:

```shell
sqly --inspect book.xlsx
```

Its `excel_sheets` array carries every sheet with `visible` (what the workbook
says) and `imported` (what this run did).

`--include-hidden-sheets` imports the hidden sheets too, and it is a session
setting: a shell started with it keeps that policy for every later `.import`.

```shell
sqly --include-hidden-sheets --sql "SELECT * FROM book_Internal" book.xlsx
```

Excel separates *hidden* (a reader can undo it from the sheet tabs) from
*very hidden* (only the VBA editor can). sqly does not distinguish them — the
library it reads workbooks with reports one flag covering both — so neither kind
is imported by default and both are imported with the flag.

There is no syntax for selecting individual sheets. Import the workbook and pick
the table you want in SQL.

An Excel source cannot be written back in place, because several tables share one file. Export to a new workbook with `--output-format excel --output`.

## Remote inputs

A positional argument may be an `http` or `https` URL, and so may the argument to
`.import`. sqly downloads it to a temporary file, imports that, and removes it
when the run ends — including when the run fails or is interrupted.

**Downloading is default-deny.** sqly makes no HTTP request unless the session
was given `--allow-remote`:

```shell
# refused before any request is made; exit 2
sqly --sql "SELECT * FROM users" https://example.com/users.csv

# allowed
sqly --allow-remote \
  --sql "SELECT * FROM users" \
  https://example.com/users.csv
```

The refusal happens before anything is resolved, so a command line mixing a
local file and a URL imports neither, creates no temporary directory, and leaves
stdout empty. The same applies to `.import URL` — typed at the prompt, piped in,
or read from a `--script-file`. In a script the whole script is checked before
its first statement runs, so a refused run has executed nothing. At the
interactive prompt the command fails, the session continues, and no table,
source record, or baseline changes.

A session started with `--allow-remote` keeps the capability for the `.import`
commands typed later in it. Granting it and not using it is not an error:
`sqly --allow-remote data.csv` and a bare `sqly --allow-remote` both run.

`--allow-remote` covers `http` and `https` only. `ftp://`, `file://`, `ssh://`,
`gopher://` and every other scheme are refused before any connection, with or
without the flag. A Windows drive path such as `C:\data\sales.csv` and a local
file name containing a colon are not URLs and are unaffected.

### What --allow-remote is not

`--allow-remote` is an explicit network capability, not a sandbox or an SSRF
defense.

It decides *whether* sqly makes an HTTP request. It decides nothing about where
that request may go. With the flag given, sqly fetches the URL it was handed,
and it does **not**:

- sandbox the process,
- prevent requests to `localhost` or to a private network range,
- prevent requests to a cloud metadata endpoint,
- defend against DNS rebinding, or re-resolve a name to check it did not move,
- prevent access through a configured proxy.

What it is for: an agent harness, a wrapper, or a CI job that fixes sqly's
argument list can leave the flag out and thereby turn sqly's own downloading
off. That is a real property, and it is the only one claimed. It is **not** a
defense against a caller who can add flags — anything that can write sqly's
command line can write `--allow-remote` onto it.

The flag is a switch on making the request, not a relaxation of anything. Every
limit below still applies with it given.

A URL is the one input nobody local vouched for: the server chooses the size and
where a redirect leads. These bounds apply to it and to nothing else.

| Bound | Value | On breach |
|:--|:--|:--|
| Download size | 2 GiB | the download is refused and the partial file removed |
| Redirects | 5 | the chain is reported as not settling |
| Redirect scheme | `http`, `https` | a redirect to any other scheme is refused by name |
| Response headers | 30 seconds | the request fails |
| Whole transfer | 15 minutes | the request fails |

The size cap holds whether or not the server declares a `Content-Length`: a
chunked response is stopped while it is being read, so a body that never ends
cannot fill the disk. A declared size over the limit is refused before the body
is read at all.

The limits are not flags. A limit that is routinely raised protects nothing.

### What the download limit does not bound

The 2 GiB cap is on the HTTP response body — the bytes that arrive over the
network. It is not a bound on what importing those bytes costs, and for some
formats the two are far apart:

- A compressed input is expanded after it lands. A 100 MB `.csv.gz` can become
  several gigabytes of CSV.
- An XLSX file is a ZIP archive, and the sheet XML inside it expands well past
  the archive's size.
- Every imported row ends up in an in-memory SQLite database. Resident memory
  runs to roughly twice the expanded data, not the downloaded size.
- Row count, column count, the size of one field, and CPU time are not capped at
  all. Formats that are parsed whole — XLSX, Parquet, JSON, LTSV — hold the
  parsed form in memory while the tables are built.

So a URL well inside 2 GiB can still exhaust memory or run for a long time.
**Treat a remote input as untrusted data**: it is chosen by whoever runs the
server, not by you. Run it where an over-large import is survivable — a
container with a memory limit, a machine you can kill — rather than relying on
the download cap to size the work.

The table is named after the file that arrives, not the URL you typed. The name
is taken from the first of these that gives a supported filename:
`Content-Disposition`, the final response URL, the URL you typed, then
`Content-Type`. A dataset behind a short link or a `latest` alias is therefore
named after its redirect target.

`HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` are honored.

A remote source cannot be written back to. `.save --in-place` refuses a table
whose source is a URL; export it to a local file instead.

## Text encodings

A text input without a Unicode BOM is decoded as UTF-8 unless `--encoding` says otherwise: `utf-8`, `shift-jis` (accepting `cp932`, `ms932`, `windows-31j`, `sjis`), `euc-jp`, `iso-2022-jp`, `utf-16le`, `utf-16be`. A BOM always wins over the flag.

## Row mismatches

When a CSV or TSV row has a different field count from the header. Only CSV and
TSV can have this problem, so `--row-mismatch` affects those two formats and no
others. It is not silently accepted for a run it could never affect: a run whose
inputs hold no CSV or TSV at all is rejected rather than exiting 0 having ignored
the flag. In a mixed run it applies to the CSV and TSV inputs and leaves the
rest alone.

| `--row-mismatch` | Behavior |
|:--|:--|
| `error` (default) | fail the import and report the row |
| `skip` | drop the row, import the rest |
| `pad` | pad a short row with empty values; fail on a long one rather than truncating it |

## Output formats

`--output-format` picks how a result is printed. The same query, three ways:

```shell
sqly --output-format table --sql "SELECT * FROM t" t.csv
```

```text
+------+-----+------+
| code | qty | note |
+------+-----+------+
|  007 |  42 |      |
+------+-----+------+
```

```shell
sqly --output-format csv --sql "SELECT * FROM t" t.csv
```

```text
code,qty,note
007,42,
```

```shell
sqly --output-format json --sql "SELECT * FROM t" t.csv
```

```json
[
  {"code":"007","qty":42,"note":""}
]
```

The three agree on every value; only JSON can express the types. A column SQLite
holds as INTEGER or REAL becomes a JSON number, TEXT becomes a JSON string, and
SQL NULL becomes `null`. Text is never re-read as a number, so `007` keeps its
leading zeros and `true` stays a string.

An empty field in a CSV is an empty string, not a NULL — above, `note` is `""`.
A NULL comes from SQL: an outer join with no match, an explicit `NULL`, an
aggregate over no rows. The two print as blank in `table` and `csv` alike, so
JSON is where they are distinguishable as `""` and `null`.

`tsv`, `ltsv`, `markdown`, `jsonl`, `vertical`, `excel`, and `parquet` are the
remaining formats; see the [reference](/reference/#output-formats).
