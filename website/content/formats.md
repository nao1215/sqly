---
title: Formats
description: Every file format sqly reads and writes, how each one becomes a table, and the compression and text encodings it handles.
weight: 50
---

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

Every sheet becomes its own table, so a workbook is queried the way a directory
is: pick the table you want. All sheets are imported — there is no flag to select
one, because selecting one is what the `FROM` clause is for.

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
| Many sheets | imports every one; a hundred sheets is a hundred tables |
| A hidden sheet | imports it like any other — hidden is a display property |
| A sheet with no cells | skips it; no empty table is created |
| A sheet with a header and no rows | imports it as a table with zero rows |
| No usable sheet at all | fails the import, saying the file produced no table |
| A sheet that cannot be read | fails the whole workbook; sqly does not import part of a file and report success |

Two workbooks may hold the same sheet name — `a.xlsx` and `b.xlsx` both with
`Sheet1` give `a_Sheet1` and `b_Sheet1`, because the file name is part of the
table name.

> **Known limitation.** Two sheets *in one workbook* whose names sanitize to the
> same table name — `Data` and `data`, or `a b` and `a.b` — currently collapse
> into one table, and the last sheet wins without a warning. Rename such sheets
> before importing.

An Excel source cannot be written back in place, because several tables share one file. Export to a new workbook with `--output-format excel --output`.

## Text encodings

A text input without a Unicode BOM is decoded as UTF-8 unless `--encoding` says otherwise: `utf-8`, `shift-jis` (accepting `cp932`, `ms932`, `windows-31j`, `sjis`), `euc-jp`, `iso-2022-jp`, `utf-16le`, `utf-16be`. A BOM always wins over the flag.

## Row mismatches

When a CSV or TSV row has a different field count from the header. Only CSV and
TSV can have this problem, so `--row-mismatch` applies to those two formats and
is ignored for the rest.

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
