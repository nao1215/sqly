#!/bin/sh
# shellcheck shell=sh
#
# --sql-file end-to-end tests (#281). --sql-file loads SQL from a file for
# non-interactive runs, which frees stdin to carry a piped --stdin dataset. The
# file supports multiline statements and the same splitting rules as batch
# stdin mode.

Describe 'sqly --sql-file (#281)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    SQL_DIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$SQL_DIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'runs a multiline query loaded from a file against a file input'
    printf -- '-- top actor by name\nSELECT actor\nFROM actor\nORDER BY actor\nLIMIT 1;\n' > "$SQL_DIR/q.sql"
    When run sqly --csv --sql-file "$SQL_DIR/q.sql" testdata/actor.csv
    The status should be success
    The output should include 'Adam Sandler'
  End

  It 'joins a piped --stdin dataset with a query loaded from a file'
    printf 'SELECT s.name, i.position\nFROM stdin s\nJOIN identifier i ON s.id = i.id\nORDER BY s.id;\n' > "$SQL_DIR/join.sql"
    Data
      #|id,name
      #|1,alice
      #|2,bob
    End
    When run sqly --stdin csv --csv --sql-file "$SQL_DIR/join.sql" testdata/identifier.csv
    The status should be success
    The output should include 'alice'
    The output should include 'developrt'
  End

  It 'runs multiple statements from a file in order'
    printf "SELECT 'first' AS x;\nSELECT 'second' AS x;\n" > "$SQL_DIR/multi.sql"
    When run sqly --csv --sql-file "$SQL_DIR/multi.sql" testdata/actor.csv
    The status should be success
    The output should include 'first'
    The output should include 'second'
  End

  Describe 'error handling'
    It 'rejects --sql and --sql-file together'
      printf 'SELECT 1;\n' > "$SQL_DIR/q.sql"
      When run sqly --sql "SELECT 1" --sql-file "$SQL_DIR/q.sql" testdata/actor.csv
      The status should be failure
      The stderr should include '--sql-file'
    End

    It 'fails for a missing SQL file'
      When run sqly --sql-file "$SQL_DIR/no_such.sql" testdata/actor.csv
      The status should be failure
      The stderr should include 'sql-file'
    End

    It 'fails for an empty SQL file'
      : > "$SQL_DIR/empty.sql"
      When run sqly --sql-file "$SQL_DIR/empty.sql" testdata/actor.csv
      The status should be failure
      The stderr should include 'empty'
    End
  End
End
