---
title: Shell
description: The sqly interactive shell — completion, history, multi-line statements, and every dot-command.
weight: 40
---

`sqly` with no `--sql` opens a REPL. Type SQL, or a helper command beginning with a dot.

![the sqly interactive shell](/img/shell-demo.gif)

```text
$ sqly testdata/user.csv
sqly v0.29.0

enter "SQL query" or "sqly command that begins with a dot".
.help print usage, .exit exit sqly.

sqly:~/sqly(table)$ SELECT * FROM user LIMIT 1;
```

The prompt shows the working directory and the current output mode. Tab completes SQL keywords, table names, column names, and paths. History persists across sessions.

## Multi-line statements

A statement is buffered until it ends with `;`, so a pasted or typed multi-line query runs as one statement. The prompt becomes `...>` while it is buffering:

```text
sqly:~/sqly(table)$ SELECT user_name,
   ...> identifier FROM user;
```

Dot-commands are single-line and run on Enter. To run a query without typing `;`, press Enter on a blank line.

## Dot-commands

### Session

| Command | Does |
|:--|:--|
| `.help` | show the command list |
| `.mode MODE` | change the output mode: `table`, `vertical`, `csv`, `tsv`, `ltsv`, `json`, `ndjson`, `json-typed`, `ndjson-typed`, `markdown`, `excel`, `parquet` |
| `.dialect [NAME]` | show or set the query dialect: `sqlite`, `mysql`, `postgresql`, `googlesql` |
| `.clear` | clear the screen |
| `.exit` | quit (so does `Ctrl-D`) |

### Navigate

| Command | Does |
|:--|:--|
| `.cd [DIR]` | change directory; no argument goes home; `~` expands |
| `.pwd` | print the working directory |
| `.ls [DIR]` | list directory contents |

### Inspect

| Command | Does |
|:--|:--|
| `.tables` | list the imported tables |
| `.schema TABLE` | print the table's `CREATE TABLE` statement |
| `.describe TABLE` | print the table's columns and types |
| `.header TABLE` | print the table's header row |

### Import and export

| Command | Does |
|:--|:--|
| `.import PATH...` | load files, directories, or `http(s)` URLs into the session |
| `.import-mode POLICY` | how to handle a ragged CSV/TSV row: `stop`, `skip`, `fill` |
| `.dump TABLE FILE` | export one table; the format follows `.mode`, or the file extension when the mode is `table` |
| `.save DIR` | write every changed table into `DIR`, leaving the sources alone |
| `.save --force` | overwrite each table's source file in place |

## Batch mode

Without a TTY, the same commands are read from stdin as a script. This is how you script sqly:

```shell
printf '.mode csv\nSELECT user_name FROM user;\n' | sqly user.csv
```

```shell
sqly user.csv <<'EOF'
.import extra.csv
SELECT COUNT(*) FROM extra;
.save ./out
EOF
```

A failing statement stops the script and exits non-zero, naming the statement and its line.

## Write-back from the shell

```text
sqly:~/data(table)$ UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1;
affected is 1 row(s)
sqly:~/data(table)$ .save ./out
Saved user to out/user.csv
```

`.save DIR` never touches the sources. `.save --force` overwrites them. A session that changed no row writes no file and says so.
