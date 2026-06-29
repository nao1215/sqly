#!/bin/sh
# shellcheck shell=sh
#
# --sql-file with --output (issue #685). A saved SQL script can export its single
# result set to a file, the same way --sql can. A script that produces zero or
# more than one result set is rejected with a clear error, and a successful
# export keeps stdout clean (the data goes only to the file).

Describe 'sqly --sql-file --output'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORKDIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$WORKDIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'exports a single-SELECT script to the output file with clean stdout'
    printf 'SELECT user_name FROM user ORDER BY identifier LIMIT 1;\n' > "$WORKDIR/q.sql"
    When run sqly --sql-file "$WORKDIR/q.sql" --output "$WORKDIR/out.csv" testdata/user.csv
    The status should be success
    # The data is written to the file, not stdout.
    The output should equal ''
    The stderr should include 'Output sql result'
    The path "$WORKDIR/out.csv" should be exist
    The contents of file "$WORKDIR/out.csv" should include 'user_name'
  End

  It 'exports a single result set even when the script first runs DDL/DML'
    printf 'CREATE TEMP TABLE picked AS SELECT user_name FROM user;\nSELECT * FROM picked ORDER BY user_name LIMIT 1;\n' > "$WORKDIR/q.sql"
    When run sqly --sql-file "$WORKDIR/q.sql" --output "$WORKDIR/out.csv" testdata/user.csv
    The status should be success
    The output should equal ''
    The stderr should include 'Output sql result'
    The path "$WORKDIR/out.csv" should be exist
  End

  It 'rejects a script that produces no result set'
    printf 'CREATE TEMP TABLE t AS SELECT 1 AS x;\n' > "$WORKDIR/q.sql"
    When run sqly --sql-file "$WORKDIR/q.sql" --output "$WORKDIR/out.csv" testdata/user.csv
    The status should be failure
    The stderr should include 'result set'
    The path "$WORKDIR/out.csv" should not be exist
  End

  It 'rejects a script that produces multiple result sets'
    printf 'SELECT user_name FROM user LIMIT 1;\nSELECT identifier FROM user LIMIT 1;\n' > "$WORKDIR/q.sql"
    When run sqly --sql-file "$WORKDIR/q.sql" --output "$WORKDIR/out.csv" testdata/user.csv
    The status should be failure
    The stderr should include 'single result set'
    The path "$WORKDIR/out.csv" should not be exist
  End
End
