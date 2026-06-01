#!/bin/sh
# shellcheck shell=sh
#
# Binary-level regressions for the v0.22.0 hardening work. These run the built
# sqly the way a user does (batch stdin, --sql, --output, files on disk, exit
# codes) so the fixes are pinned at the layer the bugs were reported against.

Describe 'sqly v0.22.0 binary regressions'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    export SQLY_HISTORY_DB_PATH="$WORK/history.db"
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # Literal dotted object names whose prefix is "main"/"temp". The quoted name is a
  # single identifier, so the literal object must win over the schema qualifier.
  It 'inspects a literal "main.x" table with .schema'
    Data
      #|CREATE TABLE "main.x"(litcol INTEGER);
      #|.schema "main.x"
    End
    When run sqly
    The status should be success
    The output should include 'litcol'
  End

  It 'inspects a literal "temp.x" table with .describe'
    Data
      #|CREATE TABLE "temp.x"(litcol INTEGER);
      #|.describe "temp.x"
    End
    When run sqly
    The status should be success
    The output should include 'litcol'
  End

  It 'inspects a literal "main.v" view with .header'
    Data
      #|CREATE VIEW "main.v" AS SELECT 1 AS litcol;
      #|.header "main.v"
    End
    When run sqly
    The status should be success
    The output should include 'litcol'
  End

  It 'exports a literal "temp.v" view with .dump'
    Data:expand
      #|CREATE VIEW "temp.v" AS SELECT 1 AS litcol;
      #|.dump "temp.v" $WORK/tv.csv
    End
    When run sqly
    The status should be success
    The output should include 'statement executed successfully'
    The stderr should be present
    The contents of file "$WORK/tv.csv" should include 'litcol'
  End

  It 'prints a paste-safe literal "main.x" name in .tables'
    Data
      #|CREATE TABLE "main.x"(litcol INTEGER);
      #|.tables
    End
    When run sqly
    The status should be success
    The output should include '"main.x"'
  End

  # Nested compression suffixes, including a long-form alias such as .gzip, must be
  # rejected before --output or .dump writes bytes under a name that lies about them.
  It 'rejects a --output destination that stacks .gzip and .zst on a format suffix'
    When run sqly --sql 'SELECT 1 AS x' --output "$WORK/fake.parquet.gzip.zst"
    The status should be failure
    The stderr should be present
    The path "$WORK/fake.parquet.gzip.zst" should not be exist
  End

  It 'rejects a --output .json.gzip.zst destination'
    When run sqly --sql 'SELECT 1 AS x' --output "$WORK/fake.json.gzip.zst"
    The status should be failure
    The stderr should be present
    The path "$WORK/fake.json.gzip.zst" should not be exist
  End

  It 'rejects a --output .ach.gzip.zst destination as input-only'
    When run sqly --sql 'SELECT 1 AS x' --output "$WORK/fake.ach.gzip.zst"
    The status should be failure
    The stderr should be present
    The path "$WORK/fake.ach.gzip.zst" should not be exist
  End

  It 'rejects a .dump destination that stacks .gzip and .zst'
    Data:expand
      #|.dump user $WORK/fake.parquet.gzip.zst
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should be present
    The path "$WORK/fake.parquet.gzip.zst" should not be exist
  End
End
