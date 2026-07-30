---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version` — `sqly help` and `sqly version` are read as input paths.

## Output formats

| Flag | Format |
|:--|:--|
| (default) | ASCII table |
| `--vertical` | one column per line, in a block per record; for rows too wide to read across |
| `-c`, `--csv` | CSV |
| `-t`, `--tsv` | TSV |
| `-l`, `--ltsv` | LTSV |
| `-j`, `--json` | JSON array, every value a string |
| `-n`, `--ndjson` | newline-delimited JSON, every value a string |
| `--json-typed` | JSON with native numbers, booleans, and nulls |
| `--ndjson-typed` | NDJSON with native numbers, booleans, and nulls |
| `-m`, `--markdown` | Markdown table |
| `-e`, `--excel` | Excel workbook (needs `--output` or `.dump`) |
| `-p`, `--parquet` | Parquet (needs `--output` or `.dump`) |

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
| `--import-mode POLICY` | ragged CSV/TSV rows: `stop` (default), `skip`, `fill` |
| `--encoding NAME` | text encoding for BOM-less input (default `utf-8`) |
| `--cache PATH` | reuse a SQLite snapshot while the inputs are unchanged |
| `--cache-clear` | delete the cache first, forcing a cold rebuild |

## Report modes

Each of these prints a report and exits, instead of running a query.

| Flag | Does |
|:--|:--|
| `-i`, `--inspect` | schema, row counts, and sample rows as JSON |
| `--inspect-sample N` | rows per table in `--inspect` (default 5; `0` for schema only) |
| `--profile` | data-quality report: nulls, blanks, distinct counts, warnings |
| `--profile-format` | `json` (default), or `text`, which leads with the columns that have warnings, then lists every column (`no warnings` when there are none) |
| `--compare` | compare two tables: schema, row count, keyed rows |
| `--compare-key COL` | key column for the keyed row comparison |
| `--compare-tables "l,r"` | which two tables to compare |
| `--compare-format` | `json` (default), or `text`, which lists the keys added (`+`), removed (`-`), and modified (`~`), and under each modified key only the columns whose value changed |

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
