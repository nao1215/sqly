---
title: Reference
description: Every sqly flag, the table name rules, and the exit codes.
weight: 60
---

sqly is flag-driven and has no subcommands. Use `sqly --help` and `sqly --version` — `sqly help` and `sqly version` are read as input paths.

The groups below are the ones `sqly --help` prints.

## Input

Positional arguments are the inputs: files, directories, and `http(s)` URLs. Each becomes a table named after the file.

| Flag | Does |
|:--|:--|
| `--stdin-format FORMAT` | read stdin as a dataset instead of as SQL: `csv`, `tsv`, `ltsv`, `json`, `jsonl` |
| `--stdin-table NAME` | table name for the `--stdin-format` dataset (default `stdin`) |
| `--sheet NAME` | import only this sheet from Excel (`.xlsx`) inputs |
| `--encoding ENCODING` | decode CSV, TSV, LTSV, JSON, and JSONL inputs that have no BOM as this encoding (default `utf-8`) |
| `--row-mismatch POLICY` | CSV/TSV only: a row whose field count differs from the header — `error` (fail the import), `skip` (drop the row), `pad` (pad a short row, fail on a long one) |

`--stdin-format` and `--stdin-table` apply to the piped dataset only. `--sheet`,
`--encoding`, and `--row-mismatch` apply to every input the run loads, including
the files inside a directory argument and the `--stdin-format` dataset, and each
is limited to the formats it can mean anything for.

| Flag | Applies to | Ignored for |
|:--|:--|:--|
| `--encoding` | CSV, TSV, LTSV, JSON, JSONL — local files, URLs, and the piped dataset alike | Excel, Parquet, ACH, Fedwire, which carry their own encoding; and the `--sql-file` script, which is always read as UTF-8 |
| `--row-mismatch` | CSV and TSV | every other format, none of which can have a field-count mismatch |
| `--sheet` | Excel (`.xlsx`) | rejected outright when no input can be an Excel file |

A Unicode BOM always wins over `--encoding`: a file that declares its encoding is
read the way it declares itself. `--stdin-table` is rejected without
`--stdin-format`, so a flag that would have no effect fails instead of being ignored.

With one workbook, a `--sheet` name that workbook does not have is an error. With
several, a workbook that does not have the sheet is skipped with a warning and the
run continues, so one non-matching workbook cannot suppress the ones that match.

## Query

| Flag | Does |
|:--|:--|
| `-s`, `--sql SQL` | run one statement, then exit |
| `-f`, `--sql-file FILE` | run every statement in a file, then exit; cannot combine with `--sql` |
| `--dialect NAME` | write the query in `sqlite` (default), `mysql`, `postgresql`, or `googlesql` and have it translated |

Without either query flag, sqly opens the interactive shell on a terminal, and
reads SQL from stdin when it is piped. `--stdin-format` takes stdin for the data,
so it needs `--sql`, `--sql-file`, or `--inspect` to say what to run.

### Scripts with several results

`--sql` runs exactly one statement; two would mean discarding a result, so it is
rejected. `--sql-file` has no such limit: every statement runs in order and every
result is printed in turn, in the chosen `--output-format`. Nothing is dropped.

`--output` writes one file, so it needs the run to produce exactly one result.
A script that produces none, or more than one, is rejected and no file is written
— rather than a file holding whichever result happened to be last.

| Run | Result |
|:--|:--|
| `--sql-file` alone | every result printed to stdout, in statement order |
| `--sql-file` + `--output` | exactly one result required; 0 or 2+ is an error and writes nothing |
| `--sql-file` + `--output-format json` | one JSON array per result; `jq` reads the concatenation as a stream, and `jsonl` avoids the question |
| `--sql-file` + `--output-format excel`/`parquet` | needs `--output`, so the one-result rule applies |

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

## Inspection

| Flag | Does |
|:--|:--|
| `--inspect` | print schema, row counts, and sample rows as JSON, then exit |
| `--inspect-sample N` | sample rows per table in `--inspect` (default 5; `0` for schema only) |

`--inspect` prints a report instead of running a query, so it is rejected together with `--sql`, `--sql-file`, `--output`, `--output-format`, and the write-back flags.

## Write back

| Flag | Does |
|:--|:--|
| `--save-tables DIR` | write every table the session changed into `DIR`, in its source format; the sources are untouched |
| `--save-in-place` | overwrite the source file of every table the session changed |

Neither writes the query result — that is `--output`. These two write the imported
tables back out in the format they were read from, which is why they are named
after the tables rather than after the output.

"changed" is measured, not assumed: sqly fingerprints each table's contents at
import and compares before writing, so a table the session did not modify is not
rewritten even when a sibling table was. A read-only run writes nothing at all and
says so on stderr.

The two are mutually exclusive, and both need `--sql`, `--sql-file`, or piped input — in the shell, `.save` does the same job once you can see what the session changed.

Only `INSERT`/`UPDATE`/`DELETE` on an imported table are persisted; a schema change is rejected before anything is written.

### What is rejected, and when

A run that could never persist is rejected before anything is imported, printed, or created:

| Rejected | Why |
|:--|:--|
| both flags together | one run cannot both keep and overwrite the sources |
| no input file or directory | there is nothing to write back to |
| a `--stdin-format` dataset | a piped dataset has no source file |
| an `http(s)` input | a remote file is not sqly's to modify |
| an input whose format has no writer (JSON, JSONL, Excel) | write-back is all-or-nothing, so one unwritable input stops the run |
| a directory argument | its tables are not a single editable source |
| an interactive session | use `.save` in the shell, where you can see what changed first |

### How a multi-file save fails

A save covering several files is all-or-nothing. Every table is written to a
scratch file beside its destination first; only when all of them have been written
are they moved into place. So a failure while encoding the second file leaves the
first one untouched, and neither the scratch files nor the backups survive the run.

If a move fails after an earlier one landed — which a rename can still do, and
which Windows forces into a copy when another handle holds the destination open —
the destinations that were already replaced are restored from backups taken before
the first move. That restore is best effort: it cannot be reported without hiding
the failure that caused it, which is the one worth showing.

An in-place save keeps the source file's permissions. It writes through a
temporary file, so without this a world-readable CSV would come back owner-only.

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
