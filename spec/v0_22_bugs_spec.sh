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

  # A leading empty statement (a bare ";") must not swallow the statement that
  # follows it in direct --sql.
  It 'runs the SELECT after a leading empty statement in direct --sql'
    When run sqly --sql ';SELECT 1 AS x'
    The status should be success
    The output should include 'x'
    The output should include '1'
  End

  It 'runs the SELECT after multiple leading empty statements in direct --sql'
    When run sqly --sql ';;SELECT 2 AS y'
    The status should be success
    The output should include '2'
  End

  It 'exports the SELECT after a leading empty statement with --output'
    When run sqly --sql ';SELECT 1 AS x' --output "$WORK/lead.csv"
    The status should be success
    The stderr should be present
    The contents of file "$WORK/lead.csv" should include 'x'
  End

  It 'still rejects ATTACH after a leading empty statement in direct --sql'
    When run sqly --sql ";ATTACH DATABASE '$WORK/x.db' AS aux"
    The status should be failure
    The stderr should include 'ATTACH'
  End

  # Write-back persists only a changed file-backed import. A session that touches
  # only a TEMP/scratch table, or makes net-zero edits, must leave the source as-is.
  # Small inline fixtures keep these fast and self-contained.
  It 'does not rewrite an unchanged CSV when only a TEMP table changed'
    Data:expand
      #|CREATE TEMP TABLE scratch(id INTEGER);
      #|INSERT INTO scratch VALUES (1);
      #|.save --force
    End
    printf 'name,age\nalice,30\nbob,25\n' > "$WORK/temp_only.csv"
    before=$(cksum < "$WORK/temp_only.csv")
    When run sqly "$WORK/temp_only.csv"
    The status should be success
    The stderr should include 'nothing to save'
    The output should not include 'Saved'
    The value "$(cksum < "$WORK/temp_only.csv")" should equal "$before"
  End

  It 'does not fail on an unchanged JSONL import when only a scratch table changed'
    Data:expand
      #|CREATE TABLE scratch(id INTEGER);
      #|INSERT INTO scratch VALUES (1);
      #|.save --force
    End
    printf '{"id":1}\n{"id":2}\n' > "$WORK/data.jsonl"
    When run sqly "$WORK/data.jsonl"
    The status should be success
    The output should include 'affected'
    The stderr should include 'nothing to save'
    The stderr should not include 'not supported'
    The stderr should not include 'not loaded from a file'
  End

  It 'does not rewrite the source after net-zero CSV edits'
    Data:expand
      #|UPDATE netzero SET age=99 WHERE name='alice';
      #|UPDATE netzero SET age=30 WHERE name='alice';
      #|.save --force
    End
    printf 'name,age\nalice,30\nbob,25\n' > "$WORK/netzero.csv"
    before=$(cksum < "$WORK/netzero.csv")
    When run sqly "$WORK/netzero.csv"
    The status should be success
    The stderr should include 'nothing to save'
    The output should not include 'Saved'
    The value "$(cksum < "$WORK/netzero.csv")" should equal "$before"
  End

  It 'does not rewrite the source after net-zero edits via --sql-file --save --force'
    printf 'name,age\nalice,30\nbob,25\n' > "$WORK/netzero_file.csv"
    printf "UPDATE netzero_file SET age=99 WHERE name='alice';\nUPDATE netzero_file SET age=30 WHERE name='alice';\n" > "$WORK/netzero.sql"
    before=$(cksum < "$WORK/netzero_file.csv")
    When run sqly --sql-file "$WORK/netzero.sql" --save --force "$WORK/netzero_file.csv"
    The status should be success
    The stderr should include 'nothing to save'
    The output should not include 'Saved'
    The value "$(cksum < "$WORK/netzero_file.csv")" should equal "$before"
  End

  It 'still persists a genuine CSV change with .save --force'
    Data:expand
      #|UPDATE genuine SET age=999 WHERE name='alice';
      #|.save --force
    End
    printf 'name,age\nalice,30\nbob,25\n' > "$WORK/genuine.csv"
    When run sqly "$WORK/genuine.csv"
    The status should be success
    The output should include 'affected'
    The stderr should include 'Saved'
    The contents of file "$WORK/genuine.csv" should include '999'
  End
End
