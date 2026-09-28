---
title: Benchmark
description: sqly against trdsql, csvq, textql and DuckDB on the same queries over the same CSV files, measured end to end with himorime.
weight: 65
---

The same query over the same CSV file, run by five tools and timed from process
start to exit with [himorime](https://github.com/nao1215/himorime). Before
timing, every tool runs the query once and the outputs are compared byte for
byte, so every bar below is the same answer.

{{< benchmark-chart >}}

## What the tools are

| Tool | Engine | Needs cgo |
|:--|:--|:--|
| sqly | SQLite, as pure Go ([modernc.org/sqlite](https://gitlab.com/cznic/sqlite)), loaded by [filesql](https://github.com/nao1215/filesql) | no |
| [trdsql](https://github.com/noborus/trdsql) | SQLite through cgo | yes |
| [csvq](https://github.com/mithrandie/csvq) | its own engine, in Go | no |
| [textql](https://github.com/dinedal/textql) | SQLite through cgo; no release since 2015 | yes |
| [DuckDB](https://duckdb.org/) | a columnar engine in C++ that reads the file in parallel | yes (C++) |

sqly, trdsql and textql load every row into SQLite before the query runs, so
their cost is mostly the load. sqly and trdsql differ in how that SQLite is
built: compiled from C for trdsql, translated into Go for sqly. In return sqly
is a single static binary for every platform Go builds for, with no C toolchain
needed. DuckDB does not load the file into a table first; it scans only the
columns the query needs, on every core, which is why it leads each benchmark by
a wide margin.

textql is left out of the export benchmark: it reads the phone number
`0389689232` as the number 389689232 and writes that back, so its output
differs from the other four.

## Results

{{< benchmark-tables >}}

## Measuring it yourself

The suite is [`bench/compare/himorime.yaml`](https://github.com/nao1215/sqly/blob/main/bench/compare/himorime.yaml).
With trdsql, csvq, textql and duckdb on `PATH`, `make bench-docs` runs it and
writes `website/data/benchmark.json`, which this page's chart and tables are
drawn from. [`bench/README.md`](https://github.com/nao1215/sqly/blob/main/bench/README.md)
pins the versions measured. Numbers from different machines are not comparable; compare tools on one
machine, as this page does.
