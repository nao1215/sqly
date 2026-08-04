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
| `.mode MODE` | change the output mode: `table`, `vertical`, `csv`, `tsv`, `ltsv`, `json`, `jsonl`, `markdown`, `excel`, `parquet` |
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
| `.row-mismatch POLICY` | how to handle a CSV/TSV row whose field count differs from the header: `error` fails the import, `skip` drops the row, `pad` fills a short row with empty values and fails on a long one |
| `.dump TABLE FILE` | export one table; the format follows `.mode`, or the file extension when the mode is `table` |
| `.save DIR` | write every changed table into `DIR`, leaving the sources alone |
| `.save --in-place` | overwrite each table's source file |
| `.save --in-place --follow-symlinks` | also overwrite through a symlinked source, which is otherwise refused |

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

A helper command must start its own line; `SELECT 1; .save ./out` is rejected,
because reading it as two things depends on knowing where the statement ended,
which is what the line does not show. Leading whitespace is fine, so a script can
indent. A `.` inside a string, a `--` comment, or a `/* */` block is part of the
statement, not a command.

A helper command runs here and in a `--script-file`, which is the same script
read from a file instead of a pipe:

```shell
sqly --script-file monthly.sqly user.csv
```

`--sql-file` holds SQL, and a dot-command in one is rejected by name and line — a
`.sql` file that runs `.save` is a shell script wearing a SQL extension. The flag
that permits side effects says `script` in its name. See the
[reference](/reference/#sql-file-or-script-file) for what each one accepts.

## Write-back from the shell

```text
sqly:~/data(table)$ UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1;
affected is 1 row(s)
sqly:~/data(table)$ .save ./out
Saved user to out/user.csv
```

`.save DIR` never touches the sources. `.save --in-place` overwrites them. A session that changed no row writes no file and says so.

Write-back lives here rather than on the command line: it is the one thing sqly does that cannot be undone, and `.save` puts it after the statements that changed something, where you can look first. The same commands work in a piped script, so a batch job writes back the same way a person does. The [reference](/reference/#write-back) has the full contract — which formats can be written, what happens when several files are saved at once, and what is guaranteed if one of them fails.
