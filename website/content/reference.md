---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version` — `sqly help` and `sqly version` are read as input paths.

## Output formats

| Flag | Format |
|:--|:--|
| `--output-format table` (default) | ASCII table |
| `--output-format vertical` | one column per line, in a block per record; for rows too wide to read across |
| `--output-format csv` | CSV |
| `--output-format tsv` | TSV |
| `--output-format ltsv` | LTSV |
| `--output-format json` | JSON array preserving SQLite numeric, text, and NULL types |
| `--output-format ndjson` | newline-delimited JSON preserving SQLite numeric, text, and NULL types |
| `--output-format markdown` | Markdown table |
| `--output-format excel` | Excel workbook (needs `--output` or `.dump`) |
| `--output-format parquet` | Parquet (needs `--output` or `.dump`) |

## Query

| Flag | Does |
|:--|:--|
| `-s`, `--sql SQL` | run one statement |
| `-f`, `--sql-file PATH` | run a script; multi-statement, cannot combine with `--sql` |
| `-o`, `--output PATH` | write the result to a file instead of stdout |
| `--dialect NAME` | write `sqlite` (default), `mysql`, `postgresql`, or `googlesql` and have it translated |

## Input

| Flag | Does |
|:--|:--|
| `-S`, `--sheet NAME` | import one Excel sheet by its original name |
| `--stdin FORMAT` | treat stdin as a dataset: `csv`, `tsv`, `ltsv`, `json`, `jsonl` |
| `--stdin-name NAME` | table name for `--stdin` (default `stdin`) |
| `--import-mode POLICY` | ragged CSV/TSV rows: `stop` (abort), `skip` (drop), `pad` (pad short rows with empty values; reject long rows without truncating) |
| `--encoding NAME` | text encoding for BOM-less input (default `utf-8`) |
| `--cache PATH` | reuse a SQLite snapshot while the inputs are unchanged (content-hash keyed) |

## Inspection

Each of these prints a report and exits, instead of running a query.

| Flag | Does |
|:--|:--|
| `-i`, `--inspect` | schema, row counts, and sample rows as JSON |
| `--inspect-sample N` | rows per table in `--inspect` (default 5; `0` for schema only) |

## Write-back

| Flag | Does |
|:--|:--|
| `--save-dir DIR` | write each changed table into `DIR`; sources untouched |
| `--save` | overwrite each source file in place; requires `--force` |
| `--force` | confirm the destructive in-place write |

Only `INSERT`/`UPDATE`/`DELETE` on an imported table are persisted; a schema change is rejected before anything is written. A run that changes no row writes no file and says so on stderr. A save covering several files is all-or-nothing: if any file cannot be written, none of them is.

Tables created by SQL, tables from a directory import, and Excel sources are rejected for write-back with an explicit error. CSV, TSV, LTSV, and Parquet sources are written individually, preserving format and compression; an ACH or Fedwire source is reconstructed as a whole from its related tables.

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

| Code | Meaning |
|:--|:--|
| `0` | success |
| `1` | an import, a query, a write, or a batch statement failed |

Errors go to stderr; query results go to stdout, so a pipeline stays clean.

## Environment

| Variable | Does |
|:--|:--|
| `SQLY_HISTORY_DB_PATH` | where the shell keeps its command history |
