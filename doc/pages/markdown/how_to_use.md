### sqly behavior

If no SQL query is specified with the `--sql` option, sqly will start the sqly shell. sqly determines the file type to be loaded from the extension when the shell starts and automatically begins importing it into the SQLite3 in-memory database. Multiple files can be loaded simultaneously. The table names will be the file names (without extensions) or the Excel sheet names. If an SQL query is specified with the `--sql` option, the SQL query result will be displayed in the terminal and sqly will exit without starting the sqly shell.

sqly allows you to change the display mode of SQL results with options. By default, the output is in table format. The output format can be changed to csv (`--csv`), tsv (`--tsv`), ltsv (`--ltsv`), markdown (`--markdown`), json (`--json`), or ndjson (`--ndjson`). Excel (`--excel`) and Parquet (`--parquet`) are export-only: they render as csv on screen and write a file only through `.dump` or `--output`. Since the output mode can be changed while the sqly shell is running, it is easy to execute `sqly sample.csv` and then change settings or execute SQL queries within the sqly shell.


### sqly options

```shell
$ sqly --help
sqly - execute SQL against CSV/TSV/LTSV/JSON/JSONL/Parquet/Excel/ACH/Fedwire with shell

[Usage]
  sqly [OPTIONS] [FILE_PATH(S)|DIRECTORY_PATH(S)]

[Example]
  - run sqly shell
    sqly
  - Execute query for csv file
    sqly --sql 'SELECT * FROM sample' ./path/to/sample.csv

[OPTIONS]
  -c, --csv             change output format to csv (default: table)
  -e, --excel           change output format to excel (default: table)
  -l, --ltsv            change output format to ltsv (default: table)
  -m, --markdown        change output format to markdown table (default: table)
  -t, --tsv             change output format to tsv (default: table)
  -j, --json            change output format to json (default: table)
  -n, --ndjson          change output format to ndjson (default: table)
  -p, --parquet         export results as parquet (export-only; use with --output or .dump)
  -S, --sheet string    excel sheet name you want to import
      --stdin string    treat stdin as an input dataset of this format (csv|tsv|ltsv|json|jsonl)
      --stdin-name string   table name for the --stdin dataset (default "stdin")
  -s, --sql string      sql query you want to execute
  -o, --output string   destination path for SQL results specified in --sql option
  -h, --help            print help message
  -v, --version         print sqly version

[LICENSE]
  MIT LICENSE - Copyright (c) 2022 CHIKAMATSU Naohiro
  https://github.com/nao1215/sqly/blob/main/LICENSE

[CONTACT]
  https://github.com/nao1215/sqly/issues

sqly runs the DB in SQLite3 in-memory mode.
So, SQL supported by sqly is the same as SQLite3 syntax.
```

### Execute sql in terminal: --sql option

`--sql` option takes an SQL statement as an optional argument.

```shell
$ sqly --sql "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id" testdata/user.csv testdata/identifier.csv 
+-----------+-----------+
| user_name | position  |
+-----------+-----------+
| booker12  | developrt |
| jenkins46 | manager   |
| smith79   | neet      |
+-----------+-----------+
```

### Pipe data into sqly: --stdin option

By default piped stdin is read as SQL and helper commands (batch mode). Use `--stdin <format>` to treat stdin as an input dataset instead. The format is given explicitly (`csv`, `tsv`, `ltsv`, `json`, or `jsonl`) because a pipe has no filename to detect it from. The table defaults to `stdin`; override it with `--stdin-name`. Piped data can be joined with file and directory arguments.

```shell
$ cat testdata/user.csv | sqly --stdin csv --sql "SELECT user_name FROM stdin LIMIT 1"
+-----------+
| user_name |
+-----------+
| booker12  |
+-----------+
```

### Command history

The sqly shell persists command history to a SQLite database under the config directory. History is best-effort: if that database cannot be created or written (for example a read-only config directory in CI or a container), sqly disables history for the session with a warning and still runs the requested query or command. Set `SQLY_HISTORY_DB_PATH` to choose a writable location.

### Change output format

```shell
$ sqly --sql "SELECT * FROM user LIMIT 2" --csv testdata/user.csv 
user_name,identifier,first_name,last_name
booker12,1,Rachel,Booker
jenkins46,2,Mary,Jenkins

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv 
[
   {
      "first_name": "Rachel",
      "identifier": "1",
      "last_name": "Booker",
      "user_name": "booker12"
   },
   {
      "first_name": "Mary",
      "identifier": "2",
      "last_name": "Jenkins",
      "user_name": "jenkins46"
   }
]

$ sqly --sql "SELECT * FROM user LIMIT 2" --json testdata/user.csv > user.json

$ sqly --sql "SELECT * FROM user LIMIT 2" --csv user.json 
first_name,identifier,last_name,user_name
Rachel,1,Booker,booker12
Mary,2,Jenkins,jenkins46
```

> [!WARNING]
> The support for JSON is limited. There is a possibility of discontinuing JSON support in the future.



### Output sql result to file

#### For linux user 

The sqly can save SQL execution results to the file using shell redirection. The --csv option outputs SQL execution results in CSV format instead of table format.

```shell
$ sqly --sql "SELECT * FROM user" --csv testdata/user.csv > test.csv
```

#### For windows user

The sqly can save SQL execution results to the file using the --output option. The --output option specifies the destination path for SQL results specified in the --sql option.

```shell
$ sqly --sql "SELECT * FROM user" --output=test.csv testdata/user.csv 
```
