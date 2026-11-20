---
title: Formats
description: Every file format sqly reads and writes, how each one becomes a table, and the compression and text encodings it handles.
weight: 50
---

## What each format can do

Every format can be read, but not every one can be written back to its source, and ACH and Excel produce more than one table per file.

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

- stdin: piped in with `--stdin-format`. Parquet, Excel, ACH, and Fedwire are binary or multi-table and have no filename to name the tables after, so they cannot come from stdin.
- Compressed: readable through `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`. ACH and Fedwire are not. A compressed Parquet file reads but cannot be written back, because Parquet already compresses internally and `.save` will not produce a doubly compressed file.
- Query result: produced by `--output-format`. Parquet and Excel are binary, so they need `--output` and are never printed.
- Write back: `.save` can rewrite the source. JSON and JSONL cannot, because the whole document lives in one column and sqly cannot rebuild the file from it. Excel cannot, because several tables share the file. ACH and Fedwire are rebuilt from their complete set of tables into one file.
- Types: `inferred` means sqly picks INTEGER, REAL, or TEXT from the values. A value that looks numeric but must stay text, such as a zero-padded code, stays text. Parquet carries its own schema, and the financial formats have theirs fixed by the specification.

A write-back keeps the compression: a `.csv.gz` source is rewritten as `.csv.gz`. A `.bz2` source cannot be written back at all, since there is no bzip2 writer, and is refused before anything is touched.

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

### CSV and TSV have no headerless mode

The first line is always the header, and nothing detects a file that has none. A headerless file loses its first row to the column names:

```shell
printf '1,alice,100\n2,bob,200\n' > nohdr.csv
sqly --inspect nohdr.csv
```

```text
row_count: 1, columns: 1, alice, 100
```

`--inspect` describes what was imported, so it also reports one row fewer than the file holds. Put a header in front of the data instead:

```shell
(echo 'c1,c2,c3'; cat nohdr.csv) | sqly --stdin-format csv --sql "SELECT * FROM stdin"
```

### Files that cannot be read at all

These two inputs are refused with exit `3`, and no flag changes that:

| Input | Message |
|:--|:--|
| a header that repeats a name | `filesql: duplicate column name: "a" (column 3)` |
| a file of zero bytes | `filesql: empty data source: file is empty` |

A file holding only a header is fine: it imports as a table with no rows.

Names are compared with surrounding whitespace removed, so `name` and ` name ` count as the same name twice. The rule is the same for CSV, TSV, LTSV, and workbook sheets.

## Write

`--output-format csv`, `--output-format tsv`, `--output-format ltsv`, `--output-format json`, `--output-format jsonl`, `--output-format markdown`, `--output-format excel`, `--output-format parquet`, and the default `table`.

`--output PATH` writes to a file. An extension sqly knows must agree with the chosen format, and `--output-format csv --output out.json` is refused as a usage error, exit `2`. An extension it does not know is written as given, so `--output report.txt` holds CSV; a path with no extension gets the format's own, so `--output report` writes `report.csv`. With the default `table` mode the format is inferred from the extension instead, falling back to CSV.

ACH and Fedwire tables can be exported to csv/tsv/xlsx like any other table. Writing them back into a valid `.ach`/`.fed` file is what `.save` does, not `--output`.

That write-back starts from the original file and applies the session's edits, because some fields of these formats exist in no table. The source file must still be readable when `.save` runs.

sqly's underlying library records where each imported file came from in `_filesql_sources`. Names beginning with `_filesql_` belong to that library: they never appear in `.tables`, and an import whose table name would start with the prefix is refused at exit `3`.

A format that cannot represent a value refuses the export. TSV and LTSV cannot hold a tab, a newline, or a carriage return inside a value. XLSX is XML, so it cannot hold any other control character, nor the noncharacters `U+FFFE` and `U+FFFF`. The error names the column, the exit code is `4`, and the destination is left as it was. csv and json carry all of these.

TSV is written without quoting, because sqly's TSV reader takes every byte between two tabs as the value. A double quote or a leading space is written as it is, and a one-column row holding an empty value is an empty line.

An export also refuses a header sqly could not read back. Names are compared with surrounding whitespace removed and again with ASCII case ignored. `SELECT * FROM a JOIN b ON a.id = b.id` names `id` twice, so it is refused in every format at exit `4`. Alias one of them:

```shell
sqly --output-format csv --output out.csv \
  --sql "SELECT a.id AS a_id, b.id AS b_id FROM a JOIN b ON a.id = b.id" a.csv b.csv
```

`ä` and `Ä` are two columns, because SQLite compares them as different.

## Compression

CSV, TSV, LTSV, JSON, JSONL, Parquet, and Excel are read through `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, and `.lz4`. `data.csv.gz` becomes table `data` with no flag.

Output compression comes from the destination's extension:

```shell
sqly --output-format csv --output out.csv.zst --sql "SELECT * FROM data" data.csv.gz
```

A write-back preserves each source's own compression.

## JSON and JSONL

A document is not split into columns. It lands whole in a `data` column, and SQLite's JSON functions read it:

```shell
sqly --sql "SELECT json_extract(data, '\$.name') AS name FROM sample" sample.jsonl
```

With the repository's `testdata/sample.jsonl`:

```text
+---------+
|  name   |
+---------+
| Alice   |
| Bob     |
| Charlie |
+---------+
```

No schema is guessed, so documents with different shapes load together, and a field missing from a row is `NULL`. See the [cookbook](/cookbook/#json-and-jsonl) for nested fields, arrays, and flattening.

## Excel

Each sheet the workbook shows becomes its own table, and you pick one in the `FROM` clause; there is no flag to select a sheet. Hidden sheets are left out unless [`--include-hidden-sheets`](#hidden-sheets) asks for them.

```shell
sqly --sql "SELECT * FROM book_Q3_actuals" book.xlsx
```

### How a sheet becomes a table name

The table name is the file's base name, an underscore, and the sheet name: `book.xlsx` with sheet `Q3_actuals` becomes `book_Q3_actuals`. Both halves are sanitized into a SQL identifier:

| In the sheet name | In the table name | Example |
|:--|:--|:--|
| Letters, digits, `_` | kept as they are, including case | `Sheet1` → `book_Sheet1` |
| Spaces, `-`, `.` | become `_` | `Q3 actuals` → `book_Q3_actuals` |
| Other punctuation (`()`, `#`, `$`, `%`, …) | dropped | `x(1)` → `book_x1` |
| A leading digit | prefixed with `sheet_` | `2024` → `book_sheet_2024` |
| Non-ASCII letters | kept, and the name then needs quoting | `売上` → `"book_売上"` |

`.tables` prints each name already quoted when quoting is required. Sheet `select` of `book.xlsx` is `book_select` and needs no quoting.

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

`a.xlsx` and `b.xlsx` can both have `Sheet1`; they give `a_Sheet1` and `b_Sheet1`.

Two sheets of one workbook can map to the same table name: `Q1 sales` and `Q1.sales` both become `book_Q1_sales`, and `x(1)` and `x1` both become `book_x1`. The import is refused before any sheet is read:

```text
$ sqly --sql "SELECT 1" book.xlsx
sheets "Q1 sales" and "Q1.sales" of book.xlsx both map to table
"book_Q1_sales"; rename one of them
```

Rename one of the sheets. Excel itself compares sheet names case-insensitively, so two sheets differing only in case cannot occur.

Only imported sheets are checked. A hidden sheet that collides with a shown one is ignored until `--include-hidden-sheets` brings it in, and then the workbook is refused.

### Hidden sheets

sqly imports only the sheets a workbook shows, since hidden sheets usually hold working-out such as lookup tables or old drafts.

An import that skipped sheets prints the count on stderr:

```text
Skipped 2 hidden sheets in book.xlsx; start sqly with --include-hidden-sheets to import them.
```

The notice gives no names. `--inspect` does:

```shell
sqly --inspect book.xlsx
```

Its `excel_sheets` array lists every sheet with `visible` (what the workbook says) and `imported` (what this run did).

`--include-hidden-sheets` imports the hidden sheets too. It is a session setting, so a shell started with it applies it to every later `.import`.

```shell
sqly --include-hidden-sheets --sql "SELECT * FROM book_Internal" book.xlsx
```

Excel has *hidden* sheets, which a reader can unhide from the sheet tabs, and *very hidden* sheets, which only the VBA editor can unhide. The library sqly reads workbooks with reports one flag for both, so neither kind is imported by default and both are imported with the flag.

An Excel source cannot be written back in place, because several tables share one file. Export to a new workbook with `--output-format excel --output`.

## Remote inputs

A positional argument may be an `http` or `https` URL, and so may the argument to `.import`. sqly downloads it to a temporary file, imports that, and removes it when the run ends, including when the run fails or is interrupted.

Downloading is default-deny. sqly makes no HTTP request unless the session was given `--allow-remote`:

```shell
# refused before any request is made; exit 2
sqly --sql "SELECT * FROM users" https://example.com/users.csv

# allowed
sqly --allow-remote \
  --sql "SELECT * FROM users" \
  https://example.com/users.csv
```

A command line mixing a local file and a URL imports neither and leaves stdout empty. `.import URL` is refused the same way whether typed, piped in, or read from a `--script-file`; a script is checked whole before its first statement runs. At the interactive prompt the command fails and the session continues unchanged.

A session started with `--allow-remote` keeps the capability for later `.import` commands. Giving the flag without a URL is not an error, and the flag supplies no input: a bare `sqly --allow-remote` with empty non-interactive stdin exits `2`, as a bare `sqly` does.

`--allow-remote` covers `http` and `https` only. `ftp://`, `file://`, `ssh://`, `gopher://` and every other scheme are refused before any connection, with or without the flag. A Windows drive path such as `C:\data\sales.csv` and a local file name containing a colon are not URLs and are unaffected.

### What --allow-remote is not

`--allow-remote` is an explicit network capability, not a sandbox or an SSRF defense. It decides whether sqly makes an HTTP request, not where the request goes. With the flag given, sqly fetches the URL it was handed and does not:

- sandbox the process,
- prevent requests to `localhost` or to a private network range,
- prevent requests to a cloud metadata endpoint,
- defend against DNS rebinding, or re-resolve a name to check it did not move,
- prevent access through a configured proxy.

An agent harness, a wrapper, or a CI job that fixes sqly's argument list can leave the flag out and so turn sqly's downloading off. That is the only property claimed. Anyone who can write sqly's command line can add `--allow-remote` to it.

The flag does not relax any of the limits below. They apply only to URLs, because the server chooses the size and where a redirect leads.

| Bound | Value | On breach |
|:--|:--|:--|
| Download size | 2 GiB | the download is refused and the partial file removed |
| Redirects | 5 | the chain is reported as not settling |
| Redirect scheme | `http`, `https` | a redirect to any other scheme is refused by name |
| Response headers | 30 seconds | the request fails |
| Whole transfer | 15 minutes | the request fails |

The size cap holds with or without a `Content-Length`: a chunked response is stopped while it is read, and a declared size over the limit is refused before the body is read. The limits are not flags.

### What the download limit does not bound

The 2 GiB cap is on the HTTP response body. What importing those bytes costs can be far larger:

- A compressed input is expanded after it lands. A 100 MB `.csv.gz` can become several gigabytes of CSV.
- An XLSX file is a ZIP archive, and the sheet XML inside expands well past the archive's size.
- Every imported row goes into an in-memory SQLite database. Resident memory runs to roughly twice the expanded data.
- Row count, column count, field size, and CPU time are not capped. XLSX, Parquet, JSON, and LTSV are parsed whole and held in memory while the tables are built.

A URL well under 2 GiB can still exhaust memory or run for a long time. Treat a remote input as untrusted data chosen by whoever runs the server, and run it where an oversized import is survivable, such as a container with a memory limit.

The table is named after the file that arrives, not the URL you typed. The name comes from the first of these that gives a supported filename: `Content-Disposition`, the final response URL, the URL you typed, then `Content-Type`. A dataset behind a short link or a `latest` alias is named after its redirect target.

`HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` are honored.

A remote source cannot be written back to. `.save --in-place` refuses a table whose source is a URL; export it to a local file instead.

## Text encodings

A text input without a Unicode BOM is decoded as UTF-8 unless `--encoding` says otherwise: `utf-8`, `shift-jis` (accepting `cp932`, `ms932`, `windows-31j`, `sjis`), `euc-jp`, `iso-2022-jp`, `utf-16le`, `utf-16be`. A BOM always wins over the flag.

### Bytes that are not UTF-8 fail the import

A text input holding bytes that are not valid UTF-8 is refused. SQLite stores TEXT as UTF-8, so such bytes would come back as mojibake, with `LENGTH`, `LIKE`, and `UPPER` working on fragments of characters.

The check is on the bytes. A Shift-JIS or EUC-JP file that is entirely ASCII is already valid UTF-8 and loads unchanged.

```shell
sqly --output-format csv --sql "SELECT * FROM sj" sj.csv
```

```text
import failed, and no table was created or changed: failed to import file sj.csv: filesql: parsing failed: failed to read CSV record: filesql: invalid UTF-8: byte 0x96 at offset 0 is not part of a valid character
hint: this file is not UTF-8. If it is Shift-JIS, EUC-JP, or another legacy encoding, load it with --encoding (one of: utf-8|shift-jis|euc-jp|iso-2022-jp|utf-16le|utf-16be), e.g. --encoding shift-jis.
```

The exit code is `3`, the import failure code. Naming the encoding reads the same file:

```shell
sqly --encoding shift-jis --output-format csv --sql "SELECT * FROM sj" sj.csv
```

sqly does not guess the encoding, because the bytes do not say and a wrong guess corrupts the data in a different way.

With `--encoding` given, the bytes are still checked against that encoding. A byte that begins nothing in it, a UTF-16 code unit cut in half, or a surrogate with no partner is refused with exit `3`:

```text
import failed, and no table was created or changed: decode sj.csv as shift-jis: byte at offset 8 is not valid shift-jis, so it would be read as the replacement character; check --encoding, one of: utf-8|shift-jis|euc-jp|iso-2022-jp|utf-16le|utf-16be
```

A `U+FFFD` stored in a UTF-16 file loads as data.

### A write-back keeps the source's encoding

`.save` rewrites a file in the encoding it was read with, as it does with compression. A Shift-JIS file edited and saved is still Shift-JIS, so the same command reads it again:

```shell
sqly --encoding shift-jis --script-file edit.sql sales.csv
sqly --encoding shift-jis --sql "SELECT * FROM sales" sales.csv
```

A value the encoding cannot write, such as an emoji in Shift-JIS, refuses the save at exit `4`, naming the encoding, and leaves the file as it was.

`--output` and `.dump` create new files, and a new file is UTF-8.

A write-back preserves values, not bytes. The text writers end every row with `\n` and quote a field only when CSV requires it, so a CRLF file saved in place comes back LF throughout, untouched rows included, and a needlessly quoted field comes back bare. If the original bytes must survive, export a copy with `--output` instead.

Parquet and Excel state their own encoding, and ACH and Fedwire are fixed-width records, so none of them is validated as UTF-8 text.

## Row mismatches

`--row-mismatch` decides what happens when a CSV or TSV row has a different field count from the header. It affects only CSV and TSV. A run with no CSV or TSV input rejects the flag instead of ignoring it; in a mixed run it applies to the CSV and TSV inputs only.

| `--row-mismatch` | Behavior |
|:--|:--|
| `error` (default) | fail the import and report the row |
| `skip` | drop the row, import the rest |
| `pad` | pad a short row with empty values; fail on a long one rather than truncating it |

The default names the other two when it stops:

```text
failed to import file rm.csv: filesql: column count mismatch: row 1 has 2 fields, want 3; use --row-mismatch skip to drop such rows, or --row-mismatch pad to fill short ones
```

Inside a running session, an `.import` suggests `.row-mismatch` instead, since the flag only applies at startup.

`skip` reports how many rows it dropped, so you know before a `.save --in-place` writes the smaller table back:

```text
warning: table "ragged": skipped 2 of 4 data rows whose field count differs from the header
```

An import that dropped nothing prints nothing.

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

Only JSON shows the types. INTEGER and REAL become JSON numbers, TEXT becomes a string, and SQL NULL becomes `null`. Text is never re-read as a number, so `007` keeps its leading zeros and `true` stays a string.

An empty CSV field is an empty string, as `note` is above. A NULL comes from SQL: an outer join with no match, an explicit `NULL`, an aggregate over no rows. `table` and `csv` print both the same way; JSON prints `""` and `null`.

In `csv` and `tsv`, a one-column result writes an empty value as `""`, because a reader would skip a blank line. Rows with several columns need no marking, since the delimiters show the field count.

`tsv`, `ltsv`, `markdown`, `jsonl`, `vertical`, `excel`, and `parquet` are the remaining formats; see the [reference](/reference/#output-formats).
