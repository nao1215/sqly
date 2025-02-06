
The sqly shell functions similarly to a common SQL client (e.g., `sqlite3` command or `mysql` command). The sqly shell has helper commands that begin with a dot. 

The sqly-shell has the following helper commands:

```shell
sqly (mode: table) > .help
      .dump: dump db table to file in a format according to output mode (default: csv)
      .exit: exit sqly
    .header: print table header
      .help: print help message
    .import: import file(s)
      .mode: change output mode
       .pwd: print current working directory
    .tables: print tables
```

### dump command

```shell
sqly (mode: table) > .dump
[Usage]
  .dump TABLE_NAME FILE_PATH
[Note]
  Output will be in the format specified in .mode.
  table mode is not available in .dump. If mode is table, .dump output CSV file.
```

### exit command

```shell
sqly (mode: table) > .exit

# the sqly shell is closed
```

### header command

```shell
sqly (mode: table) > .header
[Usage]
  .header TABLE_NAME
```

### import command

```shell
sqly (mode: table) > .import
[Usage]
  .import FILE_PATH(S) [--sheet=SHEET_NAME]

  - Supported file format: csv, tsv, ltsv, json, xlam, xlsm, xlsx, xltm, xltx
  - If import multiple files, separate them with spaces
  - Does not support importing multiple excel sheets at once
  - If import an Excel file, specify the sheet name with --sheet
```

### mode command

```shell
sqly (mode: table) > .mode
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
sqly (mode: table) > .pwd
/home/nao
```

### tables command

```shell
sqly (mode: table) > .tables
there is no table. use .import for importing file
```
