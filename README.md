<!-- ALL-CONTRIBUTORS-BADGE:START - Do not remove or modify this section -->
[![All Contributors](https://img.shields.io/badge/all_contributors-8-orange.svg?style=flat-square)](#contributors-)
<!-- ALL-CONTRIBUTORS-BADGE:END -->
  
![Coverage](https://raw.githubusercontent.com/nao1215/octocovs-central-repo/main/badges/nao1215/sqly/coverage.svg)
[![Build](https://github.com/nao1215/sqly/actions/workflows/build.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/build.yml)
[![reviewdog](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml/badge.svg)](https://github.com/nao1215/sqly/actions/workflows/reviewdog.yml)
![GitHub](https://img.shields.io/github/license/nao1215/sqly)  
[![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/nao1215/sqly/total)](https://github.com/nao1215/sqly/releases)


# sqly

sqly runs SQL against CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, and Fedwire files. It loads them into an in-memory SQLite3 database, so joins, CTEs, window functions, and aggregates all work — across formats, in one query. Compressed files (`.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`, `.s2`, `.lz4`) are read transparently.

Documentation: **https://nao1215.github.io/sqly/**

![demo](./doc/img/demo.gif)

## Try it in 30 seconds

If you have Go, paste this:

```shell
printf 'name,dept,salary\nalice,eng,120\nbob,sales,90\ncarol,eng,140\n' > staff.csv
go run github.com/nao1215/sqly@latest --sql "SELECT dept, ROUND(AVG(salary)) AS avg FROM staff GROUP BY dept" staff.csv
```

```text
+-------+-----+
| dept  | avg |
+-------+-----+
| eng   | 130 |
| sales |  90 |
+-------+-----+
```

The file is the table: `staff.csv` became `staff`. No schema to declare, no import step.

## Why sqly?

Pick the tool that fits the job:

| You want | Use |
|:--|:--|
| A field-oriented text processor for logs and columns | [awk](https://www.gnu.org/software/gawk/), [Miller](https://miller.readthedocs.io/) |
| A CSV-native SQL dialect with its own engine and cursors | [csvq](https://github.com/mithrandie/csvq) |
| SQL over CSV/TSV/JSON with a choice of backend engines | [trdsql](https://github.com/noborus/trdsql) |
| SQL over CSV with long-standing, mature tooling | [q](https://github.com/harelba/q), [textql](https://github.com/dinedal/textql) |
| A terminal UI over a DBMS and local files | [sqluv](https://github.com/nao1215/sqluv) |
| SQL over files, with an interactive shell, cross-format joins, and write-back | sqly |

sqly's emphasis is the session: an interactive shell with completion and history, files of different formats joined as peers, and the ability to write edits back into the source file.

## Install

```shell
go install github.com/nao1215/sqly@latest
```

```shell
brew install nao1215/tap/sqly
```

Arch Linux users can install the [AUR package](https://aur.archlinux.org/packages/sqly-bin):

```shell
yay -S sqly-bin
```

sqly is in the [aqua](https://aquaproj.github.io/) standard registry, and [mise](https://mise.jdx.dev/) installs it through the same registry:

```shell
aqua g -i nao1215/sqly
mise use aqua:nao1215/sqly
```

Prebuilt binaries are on the [release page](https://github.com/nao1215/sqly/releases). Runs on Windows, macOS, and Linux; building from source needs Go 1.25 or later. Releases ship cosign-signed checksums, an SPDX SBOM, and SLSA provenance — see [Install](https://nao1215.github.io/sqly/install/) for the verification commands.

## Recipes

The [cookbook](https://nao1215.github.io/sqly/cookbook/) is the fastest way in. A sample:

```shell
# Look at a file you have never seen
sqly --inspect user.csv

# Convert: csv -> json, excel, parquet, markdown, gzipped csv
sqly --json    --output user.json    --sql "SELECT * FROM user" user.csv
sqly --excel   --output user.xlsx    --sql "SELECT * FROM user" user.csv
sqly --parquet --output user.parquet --sql "SELECT * FROM user" user.csv

# Join two files, of any format, in one query
sqly --sql "SELECT u.user_name, i.position
            FROM user u JOIN identifier i ON u.identifier = i.id" user.csv.gz identifier.parquet

# Pull fields out of JSONL
sqly --sql "SELECT json_extract(data, '\$.name') AS name FROM sample" sample.jsonl

# Load a whole directory, or a URL, or a pipe
sqly ./data --sql "SELECT * FROM users"
sqly --sql "SELECT * FROM user" https://example.com/user.csv
cat user.csv | sqly --stdin csv --sql "SELECT COUNT(*) FROM stdin"

# Rank with a window function
sqly --sql "SELECT actor, RANK() OVER (ORDER BY total_gross DESC) AS rank FROM actor" actor.csv

# Find nulls, blanks, and duplicates
sqly --profile --profile-format text user.csv

# Diff two files by key
sqly --compare --compare-key id before.csv after.csv

# Edit a file in place
sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save --force user.csv

# Write MySQL, PostgreSQL, or BigQuery syntax and have it translated
sqly --dialect postgresql --sql "SELECT user_name, identifier::text FROM \"user\" WHERE user_name ILIKE 'b%'" user.csv
```

`--dialect` is translation, not emulation: constructs with no SQLite equivalent are rejected by name, and SQL that SQLite accepts is passed through, where the answer can differ from the source dialect. The [dialects page](https://nao1215.github.io/sqly/dialects/) lists both, with the divergences sqly knows about.

## The shell

`sqly` with no `--sql` opens a REPL: tab completion for keywords, tables, columns, and paths, history across sessions, and dot-commands for everything SQL has no syntax for.

![shell demo](./doc/img/shell-demo.gif)

```text
sqly:~/data(table)$ .import user.csv
sqly:~/data(table)$ SELECT user_name FROM user
   ...> WHERE identifier = 1;
sqly:~/data(table)$ .mode json
sqly:~/data(json)$ .save ./out
```

`.help` lists every command; the [shell page](https://nao1215.github.io/sqly/shell/) documents them.

## Write changes back

A session is in-memory only. `--save-dir DIR` writes each changed table into a directory and leaves the sources alone; `--save` overwrites them and requires `--force`.

![save demo](./doc/img/save-demo.gif)

```shell
sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir ./out user.csv
sqly --sql "DELETE FROM user WHERE identifier > 100" --save --force user.csv
```

Format and compression are preserved, a run that changes no row writes no file, and a save covering several files is all-or-nothing. Only row changes are persisted; a schema change is rejected before anything is written.

## Formats

| Format | Extensions | Becomes |
|:--|:--|:--|
| CSV / TSV / LTSV | `.csv` `.tsv` `.ltsv` | one table, columns from the header |
| JSON / JSONL | `.json` `.jsonl` | one table with a `data` column; query with `json_extract()` |
| Parquet | `.parquet` | one table |
| Excel | `.xlsx` | one table per sheet, named `file_sheet` |
| ACH | `.ach` | several tables (`_file_header`, `_batches`, `_entries`, `_addenda`) |
| Fedwire | `.fed` | one `_message` table |

All except ACH and Fedwire also read the compression extensions above. Text inputs without a BOM decode as UTF-8, or as Shift-JIS, EUC-JP, ISO-2022-JP, or UTF-16 with `--encoding`. See [Formats](https://nao1215.github.io/sqly/formats/).

## Flags

sqly is flag-driven and has no subcommands: use `sqly --help` and `sqly --version`, not `sqly help` or `sqly version` (those are read as input paths). Helper commands such as `.tables` and `.import` run inside the shell or batch stdin mode, not as arguments.

The [reference](https://nao1215.github.io/sqly/reference/) lists every flag, the table name rules, and the exit codes.

## Limitations

sqly runs each statement in its own transaction on an in-memory database, so a few SQLite statements are rejected with a clear error rather than failing confusingly:

- Explicit transaction control: `BEGIN`, `COMMIT`, `ROLLBACK`, `SAVEPOINT`, `RELEASE`
- `VACUUM` / `VACUUM INTO`, and `ATTACH` / `DETACH DATABASE`
- DCL such as `GRANT` / `REVOKE`

## Benchmark

`make bench` measures one full run (import the CSV into the in-memory DB, then run the query) over `testdata/benchmark/customers100000.csv` (100,000 rows, 12 columns):

| Records | Columns | Time per op | Memory per op | Allocations per op |
|--------:|--------:|------------:|--------------:|-------------------:|
| 100,000 | 12 | 515 ms | 161 MB | 2.82M |

Measured on an AMD Ryzen 7 5800U, Go 1.25, sqly v0.29.0.

The same query on the same file (top 10 countries by row count), best of 5 end-to-end runs:

| Tool | Time | Reads |
|:--|--:|:--|
| [trdsql](https://github.com/noborus/trdsql) | 0.32s | CSV, LTSV, JSON, TBLN |
| [csvq](https://github.com/mithrandie/csvq) | 0.34s | CSV, TSV, fixed-length, JSON |
| sqly | 0.49s | CSV, TSV, LTSV, JSON, JSONL, Parquet, Excel, ACH, Fedwire (+ compression) |
| [textql](https://github.com/dinedal/textql) | 0.52s | CSV, TSV |

sqly stays in the same sub-second range as the CSV-focused tools while reading the widest set of formats, shipping an interactive shell, and building as a pure-Go binary with no CGO.

## Contributing

Thanks for taking the time to contribute; see [CONTRIBUTING.md](./CONTRIBUTING.md) and [how to build and test](./doc/build_and_test.md). Contributions are not only about code: a GitHub Star also motivates development.

[![Star History Chart](https://api.star-history.com/svg?repos=nao1215/sqly&type=Date)](https://star-history.com/#nao1215/sqly&Date)

When adding features or fixing bugs, please write unit tests; sqly aims for unit-test coverage across all packages, as the tree map shows. The README demos are recorded with [charmbracelet/vhs](https://github.com/charmbracelet/vhs) from `doc/vhs/*.tape` (`make demo`), and their commands are exercised end-to-end by the atago suite in `e2e/atago/` (`make test-e2e`). The documentation site is built from `website/` with `make website`.

![treemap](./doc/img/cover-tree.svg)

Bugs and feature requests go to [GitHub Issues](https://github.com/nao1215/sqly/issues).

## Libraries used

- [filesql](https://github.com/nao1215/filesql) — the `database/sql` driver that loads and writes back every supported file format, and the dialect translation behind `--dialect`
- [prompt](https://github.com/nao1215/prompt) — the line editor behind the interactive shell

## LICENSE

The sqly project is licensed under the terms of [MIT LICENSE](./LICENSE).

## Contributors ✨

Thanks goes to these wonderful people ([emoji key](https://allcontributors.org/docs/en/emoji-key)):

<!-- ALL-CONTRIBUTORS-LIST:START - Do not remove or modify this section -->
<!-- prettier-ignore-start -->
<!-- markdownlint-disable -->
<table>
  <tbody>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://debimate.jp/"><img src="https://avatars.githubusercontent.com/u/22737008?v=4?s=75" width="75px;" alt="CHIKAMATSU Naohiro"/><br /><sub><b>CHIKAMATSU Naohiro</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Code">💻</a> <a href="https://github.com/nao1215/sqly/commits?author=nao1215" title="Documentation">📖</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Wozzardman"><img src="https://avatars.githubusercontent.com/u/128730409?v=4?s=75" width="75px;" alt="Wozzardman"/><br /><sub><b>Wozzardman</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=Wozzardman" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/edsilegxrepo"><img src="https://avatars.githubusercontent.com/u/153197739?v=4?s=75" width="75px;" alt="edsilegxrepo"/><br /><sub><b>edsilegxrepo</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=edsilegxrepo" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/Marukome0743"><img src="https://avatars.githubusercontent.com/u/146040408?v=4?s=75" width="75px;" alt="まるこめ"/><br /><sub><b>まるこめ</b></sub></a><br /><a href="https://github.com/nao1215/sqly/commits?author=Marukome0743" title="Code">💻</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/jedvardsson"><img src="https://avatars.githubusercontent.com/u/672606?v=4?s=75" width="75px;" alt="Jon Edvardsson"/><br /><sub><b>Jon Edvardsson</b></sub></a><br /><a href="https://github.com/nao1215/sqly/issues?q=author%3Ajedvardsson" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/jgstew"><img src="https://avatars.githubusercontent.com/u/2439367?v=4?s=75" width="75px;" alt="JGStew"/><br /><sub><b>JGStew</b></sub></a><br /><a href="https://github.com/nao1215/sqly/issues?q=author%3Ajgstew" title="Bug reports">🐛</a></td>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/ricardoseriani"><img src="https://avatars.githubusercontent.com/u/3369718?v=4?s=75" width="75px;" alt="Ricardo Seriani"/><br /><sub><b>Ricardo Seriani</b></sub></a><br /><a href="https://github.com/nao1215/sqly/issues?q=author%3Aricardoseriani" title="Bug reports">🐛</a></td>
    </tr>
    <tr>
      <td align="center" valign="top" width="14.28%"><a href="https://github.com/SnowleopardXI"><img src="https://avatars.githubusercontent.com/u/69493681?v=4?s=75" width="75px;" alt="Ephraim Steve Micaiah"/><br /><sub><b>Ephraim Steve Micaiah</b></sub></a><br /><a href="https://github.com/nao1215/sqly/issues?q=author%3ASnowleopardXI" title="Bug reports">🐛</a></td>
    </tr>
  </tbody>
  <tfoot>
    <tr>
      <td align="center" size="13px" colspan="7">
        <img src="https://raw.githubusercontent.com/all-contributors/all-contributors-cli/1b8533af435da9854653492b1327a23a4dbd0a10/assets/logo-small.svg">
          <a href="https://all-contributors.js.org/docs/en/bot/usage">Add your contributions</a>
        </img>
      </td>
    </tr>
  </tfoot>
</table>

<!-- markdownlint-restore -->
<!-- prettier-ignore-end -->

<!-- ALL-CONTRIBUTORS-LIST:END -->

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
