#!/bin/sh
# shellcheck shell=sh
#
# README example end-to-end tests. Each example shown in README.md is run here
# against the built binary so the documented commands and their output cannot
# silently drift from the real tool. Keep these in sync with README.md: when an
# example changes, change the matching assertion (or vice versa).

Describe 'README examples'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'Run SQL: --sql'
    It 'prints the full user table as an ASCII table'
      When run sqly --sql "SELECT * FROM user" testdata/user.csv
      The status should be success
      The output should include 'user_name'
      The output should include 'booker12'
      The output should include 'smith79'
    End

    It 'joins two files on a shared key'
      When run sqly --sql "SELECT user_name, position FROM user JOIN identifier ON user.identifier = identifier.id" testdata/user.csv testdata/identifier.csv
      The status should be success
      The output should include 'position'
      The output should include 'booker12'
      The output should include 'developrt'
    End
  End

  Describe 'Complex queries'
    It 'runs the analytics script (CTE + window + GROUP BY) from doc/vhs/analytics.sql'
      When run sqly --sql-file doc/vhs/analytics.sql testdata/actor.csv
      The status should be success
      The output should include 'Harrison Ford'
      The output should include '50+ movies'
    End

    It 'extracts JSON fields from a JSONL file via doc/vhs/json.sql'
      When run sqly --sql-file doc/vhs/json.sql testdata/sample.jsonl
      The status should be success
      The output should include 'Charlie'
      The output should include 'Nagoya'
    End

    It 'reads a gzipped CSV transparently'
      When run sqly --csv --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv.gz
      The status should be success
      The line 2 should equal 'booker12'
    End

    It 'queries a Parquet file'
      When run sqly --csv --sql "SELECT name FROM products ORDER BY CAST(price AS REAL) DESC LIMIT 1" testdata/products.parquet
      The status should be success
      The line 2 should equal 'Laptop'
    End

    It 'joins a compressed CSV with a plain CSV'
      When run sqly --csv --sql "SELECT user_name, position FROM user JOIN identifier ON user.identifier = identifier.id ORDER BY user.identifier LIMIT 1" testdata/user.csv.gz testdata/identifier.csv
      The status should be success
      The output should include 'developrt'
    End
  End

  Describe 'Output formats'
    It 'renders CSV with --csv'
      When run sqly --csv --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
      The status should be success
      The line 1 should equal 'user_name,identifier'
      The line 2 should equal 'booker12,1'
      The line 3 should equal 'jenkins46,2'
    End

    It 'renders JSON with --json'
      When run sqly --json --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
      The status should be success
      The output should include '{"user_name":"booker12","identifier":"1"}'
    End

    It 'renders NDJSON with --ndjson'
      When run sqly --ndjson --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
      The status should be success
      The line 1 should equal '{"user_name":"booker12","identifier":"1"}'
      The line 2 should equal '{"user_name":"jenkins46","identifier":"2"}'
    End

    It 'renders a markdown table with --markdown'
      When run sqly --markdown --sql "SELECT user_name, identifier FROM user LIMIT 2" testdata/user.csv
      The status should be success
      The line 1 should equal '| user_name | identifier |'
      The output should include '| booker12 | 1 |'
    End

    It 'renders LTSV with --ltsv'
      When run sqly --ltsv --sql "SELECT user_name, identifier FROM user LIMIT 1" testdata/user.csv
      The status should be success
      The line 1 should equal 'user_name:booker12	identifier:1'
    End
  End

  Describe 'Write results to a file: --output'
    It 'writes CSV to the path given by --output'
      WORK=$(mktemp -d)
      export WORK
      When run sqly --sql "SELECT * FROM user" --output "$WORK/out.csv" testdata/user.csv
      The status should be success
      The stderr should include 'out.csv'
      The contents of file "$WORK/out.csv" should include 'booker12'
      rm -rf "$WORK"
    End
  End

  Describe 'Pipe data in: --stdin'
    It 'queries piped CSV through the default stdin table'
      Data
        #|user_name,identifier,first_name,last_name
        #|booker12,1,Rachel,Booker
        #|jenkins46,2,Mary,Jenkins
        #|smith79,3,Jamie,Smith
      End
      When run sqly --stdin csv --sql "SELECT user_name FROM stdin LIMIT 1"
      The status should be success
      The output should include 'booker12'
    End

    It 'joins piped stdin with a file argument'
      Data
        #|user_name,identifier,first_name,last_name
        #|booker12,1,Rachel,Booker
        #|jenkins46,2,Mary,Jenkins
        #|smith79,3,Jamie,Smith
      End
      When run sqly --stdin csv --csv --sql "SELECT s.user_name, i.position FROM stdin s JOIN identifier i ON s.identifier = i.id" testdata/identifier.csv
      The status should be success
      The output should include 'developrt'
    End
  End

  Describe 'Batch mode'
    It 'runs a helper command and a query from piped stdin'
      Data
        #|.tables
        #|SELECT COUNT(*) FROM user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'TABLE NAME'
      The output should include 'user'
      The output should include '3'
    End
  End

  Describe 'Load SQL from a file: --sql-file'
    It 'runs SQL from a file while stdin carries a dataset'
      WORK=$(mktemp -d)
      export WORK
      printf 'SELECT s.user_name, i.position\nFROM stdin s\nJOIN identifier i ON s.identifier = i.id\nORDER BY s.identifier;\n' > "$WORK/join.sql"
      Data
        #|user_name,identifier,first_name,last_name
        #|booker12,1,Rachel,Booker
        #|jenkins46,2,Mary,Jenkins
        #|smith79,3,Jamie,Smith
      End
      When run sqly --stdin csv --sql-file "$WORK/join.sql" testdata/identifier.csv
      The status should be success
      The output should include 'developrt'
      rm -rf "$WORK"
    End
  End

  Describe 'Inspect tables: --inspect'
    It 'prints a JSON report with a stable source and column types'
      When run sqly --inspect --inspect-sample 1 testdata/identifier.csv
      The status should be success
      The output should include '"name": "identifier"'
      The output should include 'testdata/identifier.csv'
      The output should include '"type": "INTEGER"'
      The output should include '"position": "developrt"'
    End

    It 'omits sample rows with --inspect-sample 0'
      When run sqly --inspect --inspect-sample 0 testdata/user.csv
      The status should be success
      The output should include '"sample_rows": []'
    End
  End

  Describe 'Inspect schema: .schema and .describe'
    It 'prints the CREATE TABLE statement with .schema'
      Data
        #|.schema user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'CREATE TABLE "user"'
      The output should include '"identifier" INTEGER'
    End

    It 'prints column information with .describe'
      Data
        #|.describe user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'notnull'
      The output should include 'user_name'
      The output should include 'INTEGER'
    End
  End

  Describe 'Write changes back: --save-dir'
    setup() {
      WORK=$(mktemp -d)
      export WORK
      cp testdata/user.csv "$WORK/user.csv"
    }
    cleanup() { rm -rf "${WORK:-}"; }
    Before 'setup'
    After 'cleanup'

    It 'writes the updated table to --save-dir, leaving the source untouched'
      When run sqly --sql "UPDATE user SET first_name = 'Rachelle' WHERE identifier = 1" --save-dir "$WORK/out" "$WORK/user.csv"
      The status should be success
      The output should include 'affected'
      The stderr should include 'Saved user to'
      The contents of file "$WORK/user.csv" should not include 'Rachelle'
      The contents of file "$WORK/out/user.csv" should include 'Rachelle'
    End
  End

  Describe 'Directory import'
    dir_setup() {
      WORK=$(mktemp -d)
      export WORK
      cp testdata/user.csv testdata/identifier.csv "$WORK/"
    }
    dir_cleanup() { rm -rf "${WORK:-}"; }
    Before 'dir_setup'
    After 'dir_cleanup'

    It 'imports every supported file under a directory'
      Data
        #|.tables
      End
      When run sqly "$WORK"
      The status should be success
      The output should include 'user'
      The output should include 'identifier'
      The stderr should include 'Successfully imported'
    End
  End

  Describe 'ACH files'
    It 'loads ACH records into multiple tables'
      Data
        #|.tables
      End
      When run sqly testdata/ppd-debit.ach
      The status should be success
      The output should include 'ppd_debit_file_header'
      The output should include 'ppd_debit_entries'
    End

    It 'queries the entries table'
      When run sqly --csv --sql "SELECT amount FROM ppd_debit_entries" testdata/ppd-debit.ach
      The status should be success
      The output should include 'amount'
    End
  End

  Describe 'Fedwire files'
    It 'loads a Fedwire file into a single message table'
      Data
        #|.tables
      End
      When run sqly testdata/customer-transfer.fed
      The status should be success
      The output should include 'customer_transfer_message'
    End
  End
End
