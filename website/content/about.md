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
