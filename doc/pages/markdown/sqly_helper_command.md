
The sqly shell functions similarly to a common SQL client (e.g., `sqlite3` command or `mysql` command). The sqly shell has helper commands that begin with a dot. 

The sqly-shell has the following helper commands:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: change directory
     .clear: clear terminal screen
  .describe: print column information of a table
      .dump: dump db table to file in a format according to output mode (default: csv)
      .exit: exit sqly
    .header: print table header
      .help: print help message
    .import: import file(s) and/or directory(ies)
        .ls: print directory contents
      .mode: change output mode
       .pwd: print current working directory
      .save: write tables back to files: .save DIR (to a directory) or .save --force (overwrite sources)
    .schema: print CREATE TABLE statement of a table
    .tables: print tables
```

### cd command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .cd
sqly:~(table)$ .cd Desktop
sqly:Desktop(table)$ 
```

### describe command

Print one row per column (position, name, type, nullability, default, primary-key flag) for a table. Works for every imported format. In `.mode json` the output is a structured JSON array.

```shell
sqly:~/data(table)$ .describe user
+-----+------------+---------+---------+------------+----+
| cid |    name    |  type   | notnull | dflt_value | pk |
+-----+------------+---------+---------+------------+----+
|   0 | user_name  | TEXT    |       0 |            |  0 |
|   1 | identifier | INTEGER |       0 |            |  0 |
+-----+------------+---------+---------+------------+----+
```

### dump command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .dump
[Usage]
  .dump TABLE_NAME FILE_PATH
[Note]
  Output will be in the format specified in .mode.
  table mode is not available in .dump. If mode is table, .dump output CSV file.
  ACH/Fedwire tables can be dumped to csv/tsv/xlsx, but not back to .ach/.fed format.
```

### exit command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$.exit

# the sqly shell is closed
```

### header command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .header
[Usage]
  .header TABLE_NAME
```

### import command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .import
[Usage]
  .import FILE_PATH(S)|DIRECTORY_PATH(S) [--sheet=SHEET_NAME]

  - Supported file format: csv, tsv, ltsv, json, jsonl, parquet, xlsx, ach, fed
  - Compression: .gz, .bz2, .xz, .zst, .z, .snappy, .s2, .lz4 (automatically detected)
  - Files and directories can be mixed in arguments
  - Directories are automatically detected and all supported files are imported
  - If import multiple files/directories, separate them with spaces
  - For Excel files, all sheets are imported as separate tables (enables cross-sheet JOINs)
  - Use --sheet to import only a specific sheet from Excel files (works with files and directories)
  - JSON/JSONL data is stored in a 'data' column; use json_extract() to query fields
```

After `--sheet`, press TAB to complete the sheet names of the first Excel workbook on the line. Quoted and backslash-escaped names with spaces are completed in a form that stays a single argument.

### ls command

List directory contents (sorted, with a trailing `/` on directories). It runs in-process rather than calling the external `ls`/`dir`, so output is identical on every OS.

```shell
sqly:~/github/github.com/nao1215/sqly/di(table)$ .ls
wire.go
wire_gen.go
```

### schema command

Print the `CREATE TABLE` statement of a table. Works for every imported format. In `.mode json` the output is a structured `{table, schema}` object.

```shell
sqly:~/data(table)$ .schema user
CREATE TABLE "user" ("user_name" TEXT, "identifier" INTEGER, "first_name" TEXT, "last_name" TEXT)
```

### mode command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .mode
[Usage]
  .mode OUTPUT_MODE   ※ current mode=table
[Output mode list]
  table
  markdown
  csv
  tsv
  ltsv
  json
  ndjson
  json-typed ※ json output with native numbers, booleans, and nulls
  ndjson-typed ※ ndjson output with native numbers, booleans, and nulls
  excel ※ active only when executing .dump, otherwise same as csv mode
  parquet ※ active only when executing .dump, otherwise same as csv mode
```

### pwd command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .pwd
/home/nao
```

### tables command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .tables
there is no table. use .import for importing file

sqly:~/github/github.com/nao1215/sqly(table)$  .import actor.csv
sqly:~/github/github.com/nao1215/sqly(table)$  .import numeric.csv
sqly:~/github/github.com/nao1215/sqly(table)$  .tables
+------------+
| TABLE NAME |
+------------+
| actor      |
| numeric    |
+------------+
```
