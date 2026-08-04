---
title: Getting started
description: Run your first sqly query, learn how files become tables, and see the four ways to give sqly input.
weight: 20
---

## 1. The file is the table

Pass paths as arguments. Each file becomes a table named after it, minus the extension and any compression suffix.

```shell
$ sqly --sql "SELECT * FROM user" testdata/user.csv
+-----------+------------+------------+-----------+
| user_name | identifier | first_name | last_name |
+-----------+------------+------------+-----------+
| booker12  |          1 | Rachel     | Booker    |
| jenkins46 |          2 | Mary       | Jenkins   |
| smith79   |          3 | Jamie      | Smith     |
+-----------+------------+------------+-----------+
```

`user.csv.gz` is also table `user`. A name that is not a bare identifier gets quoted in queries: `2023-data.csv` becomes `sheet_2023_data`, and `売上.csv` stays `売上`. See [Table name rules](/reference/#table-name-rules).

Not sure what you have? `--inspect` prints the inferred schema, row counts, and sample rows as JSON:

```shell
sqly --inspect user.csv
```

## 2. Several files are several tables

```shell
sqly --sql "SELECT u.user_name, i.position
            FROM user u JOIN identifier i ON u.identifier = i.id" user.csv identifier.csv
```

Formats mix. A gzipped CSV joins a Parquet file joins an Excel sheet — once loaded, they are all SQLite tables.

A directory argument loads every supported file inside it.

## 3. Choose the output

The default is an ASCII table for a terminal. One flag switches it:

```shell
sqly --output-format csv      --sql "SELECT * FROM user" user.csv
sqly --output-format json     --sql "SELECT * FROM user" user.csv
sqly --output-format markdown --sql "SELECT * FROM user" user.csv
```

`--output PATH` writes to a file instead of stdout, and the extension must agree with the format:

```shell
sqly --output-format json --output user.json --sql "SELECT * FROM user" user.csv
```

Full list on the [reference page](/reference/#output-formats).

### A row too wide to read across

Every format above lays a record out along the line, and that stops working on the
files sqly was written for. A 300-column row is one 2700-character line as a table,
as CSV, as TSV, and as LTSV alike — no terminal shows it, and the column holding the
bad value has no name beside it to search for.

`--output-format vertical` turns the row on its side: one column per line, in a block per record.

```shell
sqly --output-format vertical --sql "SELECT * FROM wide LIMIT 1" wide.csv
```

```text
-[ RECORD 1 ]-----------------------------------------------
col_001 | v1
col_002 | v2
col_003 | BAD
```

Now the name and the value sit on one short line, so the bad column is one `grep`
away:

```shell
sqly --output-format vertical --sql "SELECT * FROM wide" wide.csv | grep BAD
```

In the shell it is `.mode vertical`. Vertical output is for reading, not for
parsing — it names no file format, so `.dump` and `--output` take the format from
the destination's extension, exactly as they do in table mode.

## 4. Four ways in

| Input | How |
|:--|:--|
| Files and directories | `sqly ... file.csv ./dir` |
| A URL | `sqly --allow-remote ... https://example.com/user.csv` (a URL is refused without the flag) |
| A pipe | `cat user.csv \| sqly --stdin-format csv --sql "SELECT * FROM stdin"` |
| A script on stdin | `printf '.tables\nSELECT 1;\n' \| sqly user.csv` |

Standard input does one of those jobs, never two. With `--stdin-format` it is the
data; with none of the query flags it is the script; with `--sql`, `--sql-file`,
or `--inspect` and no `--stdin-format` it is unused, and sqly says so on stderr
rather than answering as if nothing had been handed to it. See
[Reference](/reference/#what-sqly-does-with-standard-input).

## 5. The interactive shell

`sqly` with no `--sql` opens the shell. It is the same engine with completion, history, and dot-commands:

```text
$ sqly user.csv
sqly:~/data(table)$ .tables
sqly:~/data(table)$ SELECT * FROM user LIMIT 1;
sqly:~/data(table)$ .mode csv
sqly:~/data(csv)$ .exit
```

See [Shell](/shell/).

## 6. Save your edits

`UPDATE`, `INSERT`, and `DELETE` change the in-memory tables only. To persist them:

```shell
printf "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1;\n.save ./out\n" | sqly user.csv
printf "DELETE FROM user WHERE identifier > 100;\n.save --in-place\n" | sqly user.csv
```

`.save DIR` writes into a directory and leaves the sources alone. `.save --in-place` overwrites them. Either way the format, compression, and permissions of each source are preserved, a table the session did not change is not rewritten, and a save covering several files is all-or-nothing. Write-back is a shell command, not a flag, so it works the same interactively and in a piped script.

## Next

The [cookbook](/cookbook/) is the fastest way from here: recipes for converting formats, extracting JSON, querying Excel sheets, inspecting data, and writing MySQL or BigQuery syntax.
