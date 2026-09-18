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

The project's layering is checked in CI with [go-arch-lint](https://github.com/fe3dback/go-arch-lint), against the rules in [`.go-arch-lint.yml`](https://github.com/nao1215/sqly/blob/main/.go-arch-lint.yml).

## Contributing

Issues and pull requests are welcome; see [CONTRIBUTING.md](https://github.com/nao1215/sqly/blob/main/CONTRIBUTING.md) and [how to build and test](https://github.com/nao1215/sqly/blob/main/doc/build_and_test.md). A GitHub Star also motivates development.

## Benchmark

The same query on the same file, measured end to end (from starting the process to its exit) with [himorime](https://github.com/nao1215/himorime): the top 10 countries by row count of `testdata/benchmark/customers100000.csv` (100,000 rows, 12 columns), printed as CSV. Before measuring, the outputs of the four tools are compared byte for byte. The suite is [`bench/compare/himorime.yaml`](https://github.com/nao1215/sqly/blob/main/bench/compare/himorime.yaml), and `make bench-docs` measures it again and rewrites what follows, with the machine and the tool versions it ran on. Numbers from different machines are not comparable.

<!-- himorime:begin benchmarks -->

### sqly and other SQL-over-CSV tools

The top 10 countries by row count of 100 000 customers (12 columns), from reading the file to printing CSV.

#### Latency

| Benchmark | Command | Median | P95 | Mean | Stddev | Min | Max | Runs | Relative |
|---|---|--:|--:|--:|--:|--:|--:|--:|--:|
| top 10 countries 100k | sqly | 655.18ms | 683.80ms | 658.08ms | 21.70ms | 622.33ms | 685.98ms | 10 | 2.23x |
| top 10 countries 100k | trdsql | 213.00ms | 228.82ms | 214.87ms | 8.72ms | 206.43ms | 237.60ms | 10 | 0.72x |
| top 10 countries 100k | csvq | 187.17ms | 196.12ms | 187.79ms | 5.87ms | 176.88ms | 200.28ms | 10 | 0.64x |
| top 10 countries 100k | textql | 293.93ms | 315.36ms | 297.88ms | 10.06ms | 288.39ms | 318.45ms | 10 | 1.00x |

Relative is the median divided by the baseline command's median, or by the fastest command's.

#### CPU

| Benchmark | Command | User | System | Total | Total p95 | Utilization |
|---|---|--:|--:|--:|--:|--:|
| top 10 countries 100k | sqly | 879.64ms | 84.76ms | 960.98ms | 1.02s | 145.8% |
| top 10 countries 100k | trdsql | 227.69ms | 27.66ms | 255.69ms | 274.14ms | 119.9% |
| top 10 countries 100k | csvq | 559.86ms | 68.59ms | 627.10ms | 684.74ms | 335.5% |
| top 10 countries 100k | textql | 318.60ms | 22.94ms | 338.93ms | 358.93ms | 114.6% |

CPU values are medians over runs of the process tree. Utilization is CPU time divided by wall-clock time; above 100% means more than one CPU was busy. Process tree: the command plus every descendant its parent waited for (rusage); a descendant left running or reaped by init is not counted.

#### Memory

| Benchmark | Command | Peak RSS (median) | Peak RSS (max) |
|---|---|--:|--:|
| top 10 countries 100k | sqly | 187.02MiB | 189.32MiB |
| top 10 countries 100k | trdsql | 21.44MiB | 22.68MiB |
| top 10 countries 100k | csvq | 188.42MiB | 191.68MiB |
| top 10 countries 100k | textql | 31.97MiB | 33.55MiB |

Peak RSS is a resident set size, not the heap size of a language runtime. Peak RSS is the largest peak of any single process of the tree (rusage ru_maxrss), not the combined memory of processes running at the same time.

Measured with himorime v0.2.0 on linux/amd64, AMD RYZEN AI MAX+ 395 w/ Radeon 8060S (32 logical CPUs), head 94b063ed9963, seed 68008210526795.

- trdsql: github.com/noborus/trdsql v1.2.3
- csvq: csvq version 1.18.1
- textql: github.com/dinedal/textql v0.0.0-20151217051953-1785cd353c68

<!-- himorime:end benchmarks -->

sqly, trdsql and textql load the file into SQLite before running the query; csvq runs it on its own engine. sqly also reads TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH and Fedwire files, and builds without cgo.
