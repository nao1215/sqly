---
title: Shell
description: The sqly interactive shell — completion, history, multi-line statements, and every dot-command.
weight: 40
---

`sqly` with no `--sql` opens a REPL. Type SQL, or a helper command beginning with a dot.

![the sqly interactive shell](/img/shell-demo.gif)

```text
$ sqly user.csv
sqly v1.0.0

enter "SQL query" or "sqly command that begins with a dot".
.help print usage, .exit exit sqly.

sqly:~/data(table)$ SELECT * FROM user LIMIT 1;
```

The prompt shows the working directory and the current output mode. Tab
completes SQL keywords, table names, column names, and paths, and after a
dot-command it completes that command's arguments: `.dialect m` reaches `mysql`,
`.mode m` reaches `markdown`. What it offers depends on where the cursor is in
the statement; see below. History persists across sessions, and records a
line even when sqly rejected it, so the up-arrow brings it back to be fixed. It
records the line as typed, so a URL carrying credentials is stored as typed too —
`SQLY_HISTORY_PATH` says where, and the file is a plain text one with an entry
per line, readable and editable with anything (a statement typed across several
lines is stored on one, with its newlines escaped). What sqly prints back is redacted: a password
in a URL never reaches a message, a refusal, or an `--inspect` report.

## Completion reads the statement

What Tab offers depends on where the cursor is in the statement:

| Where | What is offered |
|:--|:--|
| after `FROM`, `JOIN`, `INSERT INTO`, `UPDATE` | table names; columns are left out, since none can go there |
| in a `SELECT` list, `WHERE`, `ON`, `GROUP BY`, `ORDER BY`, `SET` | columns of the tables the statement names, then the rest |
| after `alias.` or `table.` | that table's columns, spelled with the qualifier |

An alias comes from the statement's own `FROM` and `JOIN` clauses, spelled with
`AS` or without it. Typing `o.us` below and pressing Tab gives `o.user_id`, the
one column of `orders` that starts with `us`:

```text
sqly:~/data(table)$ SELECT * FROM user AS u JOIN orders o ON u.id = o.us
```

Keywords match whatever case they are typed in, so `sel` reaches `SELECT`. A
keyword inside a string literal or a comment is read as the text it is, so
`SELECT 'FROM ' || na` still completes a column.

## Multi-line statements

A statement is buffered until a `;` ends it, so a pasted or typed multi-line query runs as one statement. The prompt becomes `...>` while it is buffering:

```text
sqly:~/data(table)$ SELECT user_name,
   ...> identifier FROM user;
```

A `;` ends a statement where SQLite ends one, and nowhere else. Inside a string
literal, a quoted identifier, or a comment it is ordinary text; inside a trigger
body it separates the body's own statements without ending the `CREATE TRIGGER`
that contains them. In each case the buffer keeps collecting:

```text
sqly:~/data(table)$ CREATE TRIGGER audit_trg AFTER INSERT ON audit BEGIN
   ...>   INSERT INTO log VALUES ('written');
   ...> END;
statement executed successfully
```

A complete comment after the terminator does not delay anything: `SELECT 1; -- note`
runs on that Enter. An unclosed `/*` still buffers, because the statement after it
has not been read yet.

One line may hold several statements. Each runs in order and each result is
printed, which is what pasting a snippet usually needs:

```text
sqly:~/data(table)$ CREATE TABLE t (a); INSERT INTO t VALUES (1); SELECT a FROM t;
```

Dot-commands are single-line and run on Enter. To run a query without typing `;`, press Enter on a blank line.

A continuation line opens where the statement is, not at the margin: it keeps
the indentation of the line it continues and steps in one level for each
parenthesis that line left open.

```text
sqly:~/data(table)$ SELECT name FROM user WHERE identifier IN (
   ...>   SELECT identifier FROM orders WHERE total > (
   ...>     SELECT avg(total) FROM orders
   ...>   )
   ...> );
```

A parenthesis inside a string literal, a quoted name, or a comment is text and
indents nothing, and which is which follows the current dialect.

## Editing a long statement

`.edit` opens the last statement in your editor and runs what you save:

```text
sqly:~/data(table)$ SELECT user_name FROM user WHERE identifier = 1;
sqly:~/data(table)$ .edit
```

The editor is `$VISUAL`, or `$EDITOR` when that is unset; without either, `.edit`
says so and nothing happens. Flags come with it, so `EDITOR="code -w"` works. The
file it opens ends in `.sql`, so an editor that highlights by extension does.

What you save is echoed and run, and it may hold several statements. Saving an
empty file runs nothing, and so does leaving the editor with a non-zero status
(`:cq` in vim), which is how an editor says the edit was abandoned.

`.edit` needs a terminal to hand to the editor, so it is refused in a script or a
piped session; write the SQL in a file and use `--sql-file`.

## Keys

| Key | Does |
|:--|:--|
| `Enter` | run the statement, or continue it on the next line while it is unfinished |
| `Ctrl-C` | throw away the line being typed, or stop the statement that is running; the session stays open |
| `Ctrl-D` | quit, like `.exit` |
| `Tab` | complete keywords, tables, columns, paths, and dot-command arguments |
| `Esc` | close the completion list |
| `↑` / `↓`, `Ctrl-P` / `Ctrl-N` | walk history |
| `Ctrl-R` | search history |
| `Ctrl-A` / `Ctrl-E` | start or end of line |
| `Ctrl-F` / `Ctrl-B` | forward or back one character |
| `Ctrl-K` / `Ctrl-U` / `Ctrl-W` | delete to end of line, the whole line, the previous word |
| `Ctrl-L` | clear the screen |

Ctrl-C reaches a statement that is already running, so a query that turns out to
be slower than expected can be stopped:

```text
sqly:~/data(table)$ SELECT COUNT(*) FROM huge JOIN huge;
^C
canceled: SELECT COUNT(*) FROM huge JOIN huge;
sqly:~/data(table)$
```

A canceled statement changes nothing: its transaction rolls back, and the
session carries on. Canceling is not a failure, so a session that ends normally
afterward still exits 0.

## Dot-commands

### Session

| Command | Does |
|:--|:--|
| `.help` | show the command list |
| `.mode [MODE]` | show or set the output mode: `table`, `vertical`, `csv`, `tsv`, `ltsv`, `json`, `jsonl`, `markdown` |
| `.dialect [NAME]` | show or set the query dialect: `sqlite`, `mysql`, `postgresql`, `googlesql` |
| `.row-mismatch [POLICY]` | show or set how a CSV/TSV row whose field count differs from the header is imported: `error` fails the import, `skip` drops the row, `pad` fills a short row with empty values and fails on a long one |
| `.edit` | open the last statement in `$VISUAL` or `$EDITOR` and run what is saved |
| `.clear` | clear the screen |
| `.exit` | quit (so does `Ctrl-D`) |

`.mode`, `.dialect`, and `.row-mismatch` are the session settings. Called with
no argument each reports and succeeds:

```text
sqly:~/data(table)$ .mode
current output mode: table (available: table, vertical, csv, tsv, ltsv, json, jsonl, markdown)
```

Two of them used to fail instead, so a script that meant `.mode csv` would not
continue in the wrong mode. A value the setting does not accept is still
rejected, so that typo is still caught.

Every one of those lines goes to stderr, so a script that names its dialect
still pipes into `jq`.

`.mode` names what the screen shows, so `excel` and `parquet` are not among its
values: neither can be rendered to a terminal. Write one with a destination
instead. In a display mode (`table`, `vertical`) the extension names the format,
so `.dump TABLE out.xlsx` writes a workbook; in a mode that names a format, an
extension that disagrees is still a conflict and is rejected. From the command
line it is `--output-format excel --output FILE`.

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

### Import and export

| Command | Does |
|:--|:--|
| `.import PATH...` | load files, directories, or `http(s)` URLs into the session |
| `.dump TABLE FILE` | export one table; the format follows `.mode`, or the file extension when the mode is a display mode (`table`, `vertical`) |
| `.save DIR` | write every changed table into `DIR`, leaving the sources alone |
| `.save --in-place` | overwrite each table's source file |
| `.save --in-place --follow-symlinks` | also overwrite through a symlinked source, which is otherwise refused |

`.import` reads a workbook the way the session was started: one table per sheet
the workbook shows, unless sqly was launched with `--include-hidden-sheets`. The
sheet policy is a session setting rather than a per-import one, so it does not
change halfway through a session; see [Excel sheets](/reference/#excel-sheets).

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

A failing statement stops the script and exits non-zero, naming the statement and
its line. `--sql` carries one statement, so its failure names neither: there is
no line to return to and no run to stop.

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
