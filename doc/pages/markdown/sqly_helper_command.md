
The sqly shell functions similarly to a common SQL client (e.g., `sqlite3` command or `mysql` command). The sqly shell has helper commands that begin with a dot. 

The sqly-shell has the following helper commands:

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .help
        .cd: change directory
      .dump: dump db table to file in a format according to output mode (default: csv)
      .exit: exit sqly
    .header: print table header
      .help: print help message
    .import: import file(s)
        .ls: print directory contents
      .mode: change output mode
       .pwd: print current working directory
    .tables: print tables
```

### cd command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .cd
sqly:~(table)$ .cd Desktop
sqly:Desktop(table)$ 
```

### dump command

```shell
sqly:~/github/github.com/nao1215/sqly(table)$ .dump
[Usage]
  .dump TABLE_NAME FILE_PATH
[Note]
  Output will be in the format specified in .mode.
  table mode is not available in .dump. If mode is table, .dump output CSV file.
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
  .import FILE_PATH(S) [--sheet=SHEET_NAME]

  - Supported file format: csv, tsv, ltsv, json, xlam, xlsm, xlsx, xltm, xltx
  - If import multiple files, separate them with spaces
  - Does not support importing multiple excel sheets at once
  - If import an Excel file, specify the sheet name with --sheet
```

### ls command

ls command call the `ls` command or `dir` command in the shell.

```shell
sqly:~/github/github.com/nao1215/sqly/di(table)$ .ls
合計 8
-rw-rw-r-- 1 nao nao  661  2月  3 13:09 wire.go
-rw-rw-r-- 1 nao nao 2292  2月  7 10:40 wire_gen.go
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
  excel ※ active only when executing .dump, otherwise same as csv mode
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
