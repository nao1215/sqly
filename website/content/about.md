---
title: About
description: Why sqly exists, where the name comes from, and how it is built.
weight: 70
---

## Why sqly exists

sqly was built to make large CSV files easy to check.

In a project between 2022 and 2025, an app's master data lived in CSV files:

- large — over 20,000 rows by 300 columns, or 100,000 rows
- read by a Go program that inserted the records into several DB tables
- not one-to-one with those tables: one CSV fed several
- edited by several people, none of them engineers
- updated several times a month

Two things made that painful. Excel, Numbers, and Google Sheets take a long time to open a file that size, and often crash on it. And when a value has the wrong type — a string where a number belongs — the import fails with "a decode error occurred", without saying which column. Finding the bad column among 300 by hand, in a spreadsheet, is not an engineer's job.

So: query the file with SQL instead.

## The name

sqly was named to surpass the famous [jmoiron/sqlx](https://github.com/jmoiron/sqlx) — x, then y. That is a joke. The real origin is the slangy sense of "SQL on CSV? seriously?".

## How it is built

sqly reads each file, converts it to a table, and stores it in an in-memory SQLite3 database. It has no SQL parser of its own; parsing and execution are SQLite's, which is why the full query engine — CTEs, window functions, joins, aggregates — is available on a CSV file.

Two libraries carry most of the work, both from the same author:

- [filesql](https://github.com/nao1215/filesql) — a `database/sql` driver that loads CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, and Fedwire files into SQLite, and writes them back. It also holds the dialect translation behind `--dialect`.
- [prompt](https://github.com/nao1215/prompt) — the line editor behind the interactive shell: completion, history, multi-line input, and raw-mode handling across Unix and Windows.

The project follows Clean Architecture, checked in CI with [go-arch-lint](https://github.com/fe3dback/go-arch-lint). See [architecture](https://github.com/nao1215/sqly/blob/main/doc/architecture.md) and [design overview](https://github.com/nao1215/sqly/blob/main/doc/design_overview.md).

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](https://github.com/nao1215/sqly/blob/main/CONTRIBUTING.md) and [how to build and test](https://github.com/nao1215/sqly/blob/main/doc/build_and_test.md). A GitHub Star also motivates development.

## Benchmark

`make bench` measures one full run (import the CSV into the in-memory DB, then run the query) over `testdata/benchmark/customers100000.csv` (100,000 rows, 12 columns):

| Records | Columns | Time per op | Memory per op | Allocations per op |
|--------:|--------:|------------:|--------------:|-------------------:|
| 100,000 | 12 | 515 ms | 161 MB | 2.82M |

Measured on an AMD Ryzen 7 5800U, Go 1.25, sqly v0.30.0. The comparison below comes from the same run, so both are refreshed together rather than at each release.

The same query on the same file (top 10 countries by row count), best of 5 end-to-end runs:

| Tool | Time | Reads |
|:--|--:|:--|
| [trdsql](https://github.com/noborus/trdsql) | 0.32s | CSV, LTSV, JSON, TBLN |
| [csvq](https://github.com/mithrandie/csvq) | 0.34s | CSV, TSV, fixed-length, JSON |
| sqly | 0.49s | CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, Fedwire (+ compression) |
| [textql](https://github.com/dinedal/textql) | 0.52s | CSV, TSV |

sqly stays in the same sub-second range as the CSV-focused tools while reading the widest set of formats, shipping an interactive shell, and building as a pure-Go binary with no CGO.
