# Cookbook

Copyable one-liners. Every recipe runs as shown; swap the file names for yours.

A file becomes a table named after it, so `user.csv` is table `user`. Anything
SQLite can do, sqly can do — the file is just the table.

## Find a recipe by task

| I want to | Go to |
|:--|:--|
| Look at a file I have never seen | [First look at a file](#first-look-at-a-file) |
| Convert CSV to JSON, TSV, Excel, Parquet | [Convert between formats](#convert-between-formats) |
| Join two files | [Join across files](#join-across-files) |
| Join files of different formats | [Join across formats](#join-across-formats) |
| Read a `.gz` / `.zst` / `.xz` file | [Compressed files](#compressed-files) |
| Pull fields out of JSON or JSONL | [JSON and JSONL](#json-and-jsonl) |
| Pick one sheet out of a workbook | [Excel workbooks](#excel-workbooks) |
| Query a file on a web server | [Files over HTTP](#files-over-http) |
| Pipe data in from another command | [Pipe data in](#pipe-data-in) |
| Pipe the result into jq, awk, or sort | [Pipe data out](#pipe-data-out) |
| Load a whole directory | [Load a directory](#load-a-directory) |
| Run a saved `.sql` script | [Run SQL from a file](#run-sql-from-a-file) |
| Rank, bucket, or window over rows | [Analytics](#analytics) |
| Edit a file in place | [Write changes back](#write-changes-back) |
| Write MySQL / PostgreSQL / BigQuery SQL | [Other SQL dialects](#other-sql-dialects) |
| Handle rows with the wrong number of fields | [Ragged rows](#ragged-rows) |
| Read Shift-JIS or EUC-JP | [Text encodings](#text-encodings) |
| Speed up a repeated query | [Cache an import](#cache-an-import) |
| Query bank files (ACH, Fedwire) | [Financial formats](#financial-formats) |
| Use it in a script or a pipeline | [Scripting](#scripting) |

## First look at a file

Print the whole table:

```shell
sqly --sql "SELECT * FROM user" user.csv
```

Print the schema sqly inferred, the row count, and a few sample rows, as JSON:

```shell
sqly --inspect user.csv
```

Schema only, no sample rows:

```shell
sqly --inspect --inspect-sample 0 user.csv
```

Count the rows:

```shell
sqly --sql "SELECT COUNT(*) AS rows FROM user" user.csv
```

See the columns and their types without leaving the shell:

```shell
sqly user.csv
sqly:~(table)$ .schema user
sqly:~(table)$ .describe user
```

## Convert between formats

The output flag decides the format; `--output` decides where it goes.

```shell
sqly --output-format json   --sql "SELECT * FROM user" --output user.json    user.csv
sqly --output-format ndjson --sql "SELECT * FROM user" --output user.jsonl   user.csv
sqly --output-format tsv    --sql "SELECT * FROM user" --output user.tsv     user.csv
sqly --output-format ltsv   --sql "SELECT * FROM user" --output user.ltsv    user.csv
sqly --output-format excel  --sql "SELECT * FROM user" --output user.xlsx    user.csv
sqly --output-format parquet --sql "SELECT * FROM user" --output user.parquet user.csv
sqly --output-format markdown --sql "SELECT * FROM user" --output user.md    user.csv
sqly --output-format csv    --sql "SELECT * FROM user" --output user.csv.gz  user.tsv
```

The other direction is the same command with the input swapped:

```shell
sqly --output-format csv --sql "SELECT * FROM user" --output user.csv user.parquet
```

Numbers, booleans, and nulls are emitted as native JSON scalars. Values that
are not unambiguous JSON scalars, such as zero-padded identifiers, remain
strings:

```shell
sqly --output-format json --sql "SELECT identifier, user_name FROM user" user.csv
```

Compression comes from the destination's extension — `.gz`, `.bz2`, `.xz`,
`.zst`, `.z`, `.snappy`, `.s2`, `.lz4`:

```shell
sqly --output-format csv --sql "SELECT * FROM user" --output user.csv.zst user.csv
```

## Join across files

Two files, one query:

```shell
sqly --sql "SELECT u.user_name, i.position
            FROM user u JOIN identifier i ON u.identifier = i.id" user.csv identifier.csv
```

Left join to keep unmatched rows:

```shell
sqly --sql "SELECT u.user_name, i.position
            FROM user u LEFT JOIN identifier i ON u.identifier = i.id" user.csv identifier.csv
```

Anti-join — rows in the left file with no match on the right:

```shell
sqly --sql "SELECT * FROM user
            WHERE identifier NOT IN (SELECT id FROM identifier)" user.csv identifier.csv
```

Set difference between two files with the same shape:

```shell
sqly --sql "SELECT * FROM today EXCEPT SELECT * FROM yesterday" today.csv yesterday.csv
```

## Join across formats

Format is irrelevant once the file is a table:

```shell
sqly --sql "SELECT o.id, p.name
            FROM orders o JOIN products p ON o.product_id = p.id" orders.csv.gz products.parquet
```

```shell
sqly --sql "SELECT * FROM sheet1 JOIN log ON sheet1.id = log.id" book.xlsx log.jsonl
```

## Compressed files

Nothing to declare — sqly reads `.gz`, `.bz2`, `.xz`, `.zst`, `.z`, `.snappy`,
`.s2`, and `.lz4` transparently, and the table is named after the file without
its compression suffix:

```shell
sqly --sql "SELECT * FROM access" access.csv.gz
```

Round-trip through a different codec:

```shell
sqly --output-format csv --sql "SELECT * FROM access" --output access.csv.zst access.csv.gz
```

## JSON and JSONL

A JSON or JSONL file lands in one `data` column holding the document. Use
SQLite's JSON functions on it:

```shell
sqly --sql "SELECT json_extract(data, '\$.name') AS name,
                   json_extract(data, '\$.age')  AS age
            FROM sample WHERE json_extract(data, '\$.age') >= 30" sample.jsonl
```

Nested fields and arrays:

```shell
sqly --sql "SELECT json_extract(data, '\$.address.city') AS city,
                   json_extract(data, '\$.tags[0]')      AS first_tag
            FROM sample" sample.json
```

Explode an array into rows with `json_each`:

```shell
sqly --sql "SELECT json_extract(s.data, '\$.name') AS name, t.value AS tag
            FROM sample s, json_each(s.data, '\$.tags') t" sample.jsonl
```

Flatten JSONL into a CSV:

```shell
sqly --output-format csv --output flat.csv \
     --sql "SELECT json_extract(data, '\$.id')   AS id,
                   json_extract(data, '\$.name') AS name
            FROM sample" sample.jsonl
```

## Excel workbooks

Every sheet becomes a table named `filename_sheetname`:

```shell
sqly --inspect book.xlsx
sqly --sql "SELECT * FROM book_Sheet1" book.xlsx
```

Import one sheet by its original name:

```shell
sqly --sheet "Q3 actuals" --sql "SELECT * FROM book_Q3_actuals" book.xlsx
```

Write a result back out as a workbook:

```shell
sqly --output-format excel --output summary.xlsx \
     --sql "SELECT region, SUM(amount) AS total FROM sales GROUP BY region" sales.csv
```

## Files over HTTP

An `http://` or `https://` argument is downloaded and then imported:

```shell
sqly --sql "SELECT * FROM user" https://example.com/data/user.csv
```

The same URL works from the shell:

```shell
sqly:~(table)$ .import https://example.com/data/user.csv
```

Only `http` and `https` are fetched; any other scheme is rejected by name.

## Pipe data in

`--stdin` names the format of whatever is on standard input; the table is called
`stdin` unless `--stdin-name` says otherwise:

```shell
cat user.csv | sqly --stdin csv --sql "SELECT * FROM stdin"
```

```shell
curl -s https://example.com/user.csv | sqly --stdin csv --sql "SELECT COUNT(*) FROM stdin"
```

Join a pipe with a file on disk:

```shell
cat orders.csv | sqly --stdin csv --stdin-name orders \
  --sql "SELECT o.id, p.name FROM orders o JOIN products p ON o.pid = p.id" products.csv
```

With no TTY and no `--sql`, stdin is read as a script instead — SQL and
dot-commands, one per line:

```shell
printf '.tables\nSELECT COUNT(*) FROM user;\n' | sqly user.csv
```

## Pipe data out

sqly's non-table output is meant for the next command in the pipe. `--output-format ndjson`
gives one object per line, which is what `jq` reads without buffering the whole
result:

```shell
sqly --output-format ndjson --sql "SELECT path, status FROM logs WHERE status >= 500" logs.csv | jq -r '.path'
```

Filtering in SQL before `jq` shapes means `jq` only sees the rows that matter,
which is the division of labour worth reaching for on a large file — SQL has the
`WHERE`, `GROUP BY`, and `JOIN`; `jq` has the string formatting:

```shell
sqly --output-format ndjson --sql "SELECT json_extract(data,'\$.id') AS id, json_extract(data,'\$.user.name') AS name FROM events WHERE json_extract(data,'\$.level') = 'error'" events.jsonl | jq -r '"\(.id):\(.name)"'
```

For nested JSON, sqly can replace `jq` outright: `json_extract` reaches into the
document, and SQL does the aggregation `jq` makes hard. See
[JSON and JSONL](#json-and-jsonl).

`--output-format tsv` is the format for the classic text tools, because a tab is the field
separator `cut`, `awk`, and `sort -k` already expect. `tail -n +2` drops the
header:

```shell
sqly --output-format tsv --sql "SELECT status, path FROM logs" logs.csv | tail -n +2 | cut -f1 | sort -rn | head -n 1
```

sqly reads and writes the same pipe, so it can sit in the middle of one:

```shell
cat sales.csv | sqly --output-format csv --stdin csv --stdin-name s --sql "SELECT region FROM s WHERE amount > 75" | sort -u
```

A compressed source needs no decompression stage in front of it:

```shell
sqly --output-format csv --sql "SELECT COUNT(*) FROM sales" sales.csv.gz
```

### Exit codes in a pipeline

sqly exits non-zero when an import or a query fails, so `set -e` stops the script:

```shell
set -e
sqly --output-format csv --sql "SELECT * FROM no_such_table" logs.csv
echo "not reached"
```

One shell rule to know: a pipeline's status is its **last** command's, so a
failing sqly in `sqly ... | cat` does not stop anything — `$?` is `cat`'s zero.
Put sqly last, capture it in a variable, or use a shell with `pipefail`:

```shell
blank=$(sqly --output-format csv --sql "SELECT COUNT(*) FROM users WHERE email = ''" users.csv | tail -n 1)
if [ "$blank" != "0" ]; then
  echo "found $blank rows with no email" >&2
  exit 1
fi
```

## Load a directory

Point sqly at a directory and every supported file in it becomes a table:

```shell
sqly ./data --sql "SELECT * FROM users JOIN orders ON users.id = orders.user_id"
```

Files and directories mix freely:

```shell
sqly ./data extra.csv --sql "SELECT * FROM extra"
```

## Run SQL from a file

```shell
sqly --sql-file report.sql sales.csv
```

The script may hold several statements; each result is printed in turn. Send a
single-result script straight to a file:

```shell
sqly --output-format csv --sql-file report.sql --output report.csv sales.csv
```

## Analytics

Rank with a window function:

```shell
sqly --sql "SELECT actor, total_gross,
                   RANK() OVER (ORDER BY total_gross DESC) AS rank
            FROM actor ORDER BY rank LIMIT 5" actor.csv
```

Running total:

```shell
sqly --sql "SELECT day, amount,
                   SUM(amount) OVER (ORDER BY day) AS running
            FROM sales" sales.csv
```

Bucket rows with `CASE`:

```shell
sqly --sql "SELECT CASE WHEN n >= 50 THEN '50+'
                        WHEN n >= 35 THEN '35-49'
                        ELSE 'under 35' END AS bucket,
                   COUNT(*) AS rows
            FROM actor GROUP BY bucket" actor.csv
```

A CTE feeding an aggregate:

```shell
sqly --sql "WITH per_region AS (
              SELECT region, SUM(amount) AS total FROM sales GROUP BY region
            )
            SELECT region, total FROM per_region WHERE total > 1000 ORDER BY total DESC" sales.csv
```

Deduplicate, keeping the first row per key:

```shell
sqly --sql "SELECT * FROM (
              SELECT *, ROW_NUMBER() OVER (PARTITION BY email ORDER BY id) AS rn FROM users
            ) WHERE rn = 1" users.csv
```

Cross join to build a matrix:

```shell
sqly --sql "SELECT a.name, b.name FROM team a CROSS JOIN team b WHERE a.name < b.name" team.csv
```

Find duplicates by hand:

```shell
sqly --sql "SELECT email, COUNT(*) AS n FROM users GROUP BY email HAVING n > 1" users.csv
```

Find rows with a missing field:

```shell
sqly --sql "SELECT * FROM users WHERE TRIM(email) = ''" users.csv
```

## Write changes back

A session is in-memory only. `--save-dir` writes each changed table into a
directory and leaves the originals alone:

```shell
sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir ./out user.csv
```

`--save` overwrites the source files, and requires `--force`:

```shell
sqly --sql "DELETE FROM user WHERE identifier > 100" --save --force user.csv
```

Compression and format are preserved, so a `.csv.gz` source is rewritten as
`.csv.gz`. A run that changes no row writes no file and says so. A save covering
several files is all-or-nothing.

From the shell:

```shell
sqly:~(table)$ UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1;
sqly:~(table)$ .save ./out
sqly:~(table)$ .save --force
```

Dump one table to a file without ending the session:

```shell
sqly:~(table)$ .mode json
sqly:~(table)$ .dump user user.json
```

## Other SQL dialects

`--dialect` lets you write MySQL, PostgreSQL, or GoogleSQL (BigQuery / Cloud
Spanner) and have it translated to SQLite before it runs:

```shell
sqly --dialect mysql      --sql "SELECT \`user_name\`, IF(\`identifier\` = 1, 'first', 'other') FROM \`user\`" user.csv
sqly --dialect postgresql --sql "SELECT user_name, identifier::text FROM \"user\" WHERE user_name ILIKE 'b%'" user.csv
sqly --dialect googlesql  --sql "SELECT SAFE_DIVIDE(total, n) FROM stats" stats.csv
```

Switch it mid-session:

```shell
sqly:~(table)$ .dialect mysql
sqly:~(table)$ .dialect
```

Loading files always uses SQLite; only the queries you write are translated.

Translation is best effort: a construct with no SQLite equivalent is rejected by
name, but SQL that SQLite accepts is passed through and can answer differently
from the source dialect without any error. See the dialects page for what is
translated, what is rejected, and what diverges.

## Ragged rows

A CSV or TSV row whose field count differs from the header stops the import by
default. `--import-mode` chooses otherwise:

```shell
sqly --import-mode stop --sql "SELECT * FROM data" data.csv   # default: fail
sqly --import-mode skip --sql "SELECT * FROM data" data.csv   # drop the row
sqly --import-mode pad --sql "SELECT * FROM data" data.csv    # pad short rows; reject long rows
```

## Text encodings

A file without a Unicode BOM is read as UTF-8 unless told otherwise:

```shell
sqly --encoding shift-jis --sql "SELECT * FROM sales" sales.csv
sqly --encoding euc-jp     --sql "SELECT * FROM sales" sales.csv
sqly --encoding utf-16le   --sql "SELECT * FROM sales" sales.csv
```

Accepted: `utf-8`, `shift-jis` (and the `cp932`/`windows-31j` aliases),
`euc-jp`, `iso-2022-jp`, `utf-16le`, `utf-16be`. A BOM always wins.

## Cache an import

Loading a large file repeatedly is the slow part. `--cache` keeps a SQLite
snapshot and reuses it while the inputs are unchanged:

```shell
sqly --cache ./big.cache --sql "SELECT COUNT(*) FROM big" big.csv
sqly --cache ./big.cache --sql "SELECT * FROM big LIMIT 10" big.csv   # reused
```

The cache is keyed by each input's path, size, and content hash, so editing the
source rebuilds it automatically. There is no manual clear operation.

## Financial formats

An ACH file becomes several related tables; a Fedwire file becomes one:

```shell
sqly --inspect payment.ach
sqly --sql "SELECT individual_name, amount FROM payment_entries" payment.ach
sqly --sql "SELECT amount, beneficiary_name FROM transfer_message" transfer.fed
```

Edits are written back into a valid file, with the whole set reconstructed:

```shell
sqly --sql "UPDATE payment_entries SET individual_name = 'NEW NAME' WHERE entry_index = 0" \
     --save --force payment.ach
```

## Scripting

Machine-readable output, no table borders:

```shell
count=$(sqly --output-format csv --sql "SELECT COUNT(*) FROM user" user.csv | tail -n 1)
```

Fail a script when a check finds anything:

```shell
if [ "$(sqly --output-format csv --sql "SELECT COUNT(*) FROM users WHERE email = ''" users.csv | tail -n 1)" != "0" ]; then
  echo "found rows with no email" >&2
  exit 1
fi
```

Feed the result to another tool:

```shell
sqly --output-format json --sql "SELECT * FROM user" user.csv | jq '.[].user_name'
```

Batch mode from a heredoc:

```shell
sqly user.csv <<'EOF'
.mode csv
SELECT user_name FROM user ORDER BY identifier;
EOF
```

sqly exits non-zero when an import or a query fails, so `set -e` works as
expected.
