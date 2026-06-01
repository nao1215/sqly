#!/bin/sh
# shellcheck shell=sh
#
# Binary-level regressions for the v0.21.0 hardening work. These run the built
# sqly the way a user does (flags, batch stdin, --sql-file, exit codes, stdout vs
# stderr, files on disk) so the fixes are pinned at the layer the bugs were
# reported against.

Describe 'sqly v0.21.0 binary regressions'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    export SQLY_HISTORY_DB_PATH="$WORK/history.db"
    cp "$PROJECT_ROOT/testdata/user.csv" "$WORK/user.csv"
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # Helper-command name resolution: temp-before-main, TEMP keyword, dotted literal
  # names, paste-safe quoting.
  It 'prefers a TEMP table over a same-named main table in .schema'
    Data
      #|CREATE TEMP TABLE user(id TEXT);
      #|.schema user
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The output should include 'TEMP'
    The output should not include 'first_name'
  End

  It 'prefers a TEMP view over a same-named main table in .schema'
    Data
      #|CREATE TEMP VIEW user AS SELECT 1 AS id;
      #|.schema user
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The output should include 'TEMP VIEW'
  End

  It 'keeps both a main and a same-named TEMP object in .tables'
    Data
      #|CREATE TEMP TABLE user(id TEXT);
      #|.tables
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The output should include 'temp.user'
  End

  It 'targets a literal dotted table name in .schema'
    Data
      #|CREATE TABLE "a.b"(id INTEGER);
      #|.schema "a.b"
    End
    When run sqly
    The status should be success
    The output should include 'id'
  End

  It 'targets a literal dotted table name in .describe'
    Data
      #|CREATE TABLE "a.b"(id INTEGER);
      #|.describe "a.b"
    End
    When run sqly
    The status should be success
    The output should include 'id'
  End

  It 'targets a literal dotted table name in .header'
    Data
      #|CREATE TABLE "a.b"(id INTEGER);
      #|.header "a.b"
    End
    When run sqly
    The status should be success
    The output should include 'id'
  End

  It 'targets a literal dotted table name in .dump'
    Data:expand
      #|CREATE TABLE "a.b"(id INTEGER);
      #|.dump "a.b" $WORK/ab.csv
    End
    When run sqly
    The status should be success
    The output should include 'statement executed successfully'
    The stderr should be present
    The contents of file "$WORK/ab.csv" should include 'id'
  End

  It 'prints a paste-safe quoted identifier in .tables'
    Data
      #|CREATE TABLE "two words"(id INTEGER);
      #|.tables
    End
    When run sqly
    The status should be success
    The output should include '"two words"'
  End

  It 'keeps the full spaced table name in .header'
    Data
      #|CREATE TABLE "two words"(id INTEGER);
      #|.header "two words"
    End
    When run sqly
    The status should be success
    The output should include 'two words'
  End

  It 'keeps the TEMP keyword for a temp-qualified table in .schema'
    Data
      #|CREATE TEMP TABLE t(id INTEGER PRIMARY KEY);
      #|.schema temp.t
    End
    When run sqly
    The status should be success
    The output should include 'TEMP'
  End

  It 'keeps the TEMP keyword for a temp-qualified view in .schema'
    Data
      #|CREATE TEMP VIEW v AS SELECT 1 AS id;
      #|.schema temp.v
    End
    When run sqly
    The status should be success
    The output should include 'TEMP VIEW'
  End

  # Direct --sql runs exactly one statement; multi-statement input is rejected.
  Describe 'direct --sql rejects multi-statement input'
    Parameters
      "SELECT 1 AS x; SELECT 2 AS y"
      "SELECT 1 AS x; UPDATE user SET first_name='z'"
    End

    It "rejects: $1"
      When run sqly --sql "$1" "$WORK/user.csv"
      The status should be failure
      The stderr should include 'single SQL statement'
    End
  End

  It 'rejects multi-statement --sql --output before writing the file'
    When run sqly --csv --sql "SELECT 1 AS x; SELECT 2 AS y" --output "$WORK/out.csv"
    The status should be failure
    The stderr should include 'single SQL statement'
    The path "$WORK/out.csv" should not be exist
  End

  # --save / --save-dir reject side-effecting PRAGMA statements that cannot be
  # written back.
  Describe 'save mode rejects non-persistable PRAGMA statements'
    Parameters
      "PRAGMA user_version=1"
      "PRAGMA incremental_vacuum"
      "PRAGMA journal_mode=OFF"
    End

    It "rejects under --save --force: $1"
      When run sqly --sql "$1" --save --force "$WORK/user.csv"
      The status should be failure
      The stderr should be present
      The output should not include 'journal_mode'
    End
  End

  It 'rejects a setter PRAGMA under --save-dir and writes nothing'
    When run sqly --sql "PRAGMA user_version=1" --save-dir "$WORK/out" "$WORK/user.csv"
    The status should be failure
    The stderr should be present
    The path "$WORK/out" should not be exist
  End

  It 'rejects a command PRAGMA under --save-dir and writes nothing'
    When run sqly --sql "PRAGMA incremental_vacuum" --save-dir "$WORK/out" "$WORK/user.csv"
    The status should be failure
    The stderr should be present
    The path "$WORK/out" should not be exist
  End

  # END (an alias for COMMIT) is rejected as unsupported transaction control on
  # every execution path.
  It 'rejects END in direct --sql'
    When run sqly --sql "END"
    The status should be failure
    The stderr should include 'transaction'
  End

  It 'rejects END in batch stdin'
    Data
      #|END;
    End
    When run sqly
    The status should be failure
    The stderr should include 'transaction'
  End

  It 'rejects END in a --sql-file script'
    printf 'END;\n' > "$WORK/end.sql"
    When run sqly --sql-file "$WORK/end.sql"
    The status should be failure
    The stderr should include 'transaction'
  End

  # Nested compression suffixes are rejected instead of writing a mislabeled file.
  Describe '--output rejects nested compression suffixes and writes nothing'
    Parameters
      "out.csv.gz.zst"
      "out.parquet.gz.zst"
      "out.xlsx.gz.zst"
    End

    It "rejects: $1"
      When run sqly --sql "SELECT 1 AS x" --output "$WORK/$1"
      The status should be failure
      The stderr should be present
      The path "$WORK/$1" should not be exist
    End
  End

  It 'rejects a nested .dump destination and writes nothing'
    Data:expand
      #|.dump user $WORK/d.csv.gz.zst
    End
    When run sqly "$WORK/user.csv"
    The status should be failure
    The stderr should be present
    The path "$WORK/d.csv.gz.zst" should not be exist
  End

  # .tables and .header honor structured output modes.
  It 'emits structured .tables output under .mode json'
    Data
      #|.mode json
      #|.tables
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The stderr should include 'Change output mode'
    The output should include '"name"'
    The output should include '"schema"'
  End

  It 'emits structured .header output under .mode ndjson'
    Data
      #|.mode ndjson
      #|.header user
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The stderr should include 'Change output mode'
    The output should include '"column"'
    The output should include 'first_name'
  End

  # A read-only session writes back nothing, so .save does not rewrite sources or
  # emit fresh exports.
  It 'does not rewrite an unchanged source on .save --force'
    printf 'user_name,identifier\nalice,1' > "$WORK/ro.csv"
    Data
      #|.save --force
    End
    When run sqly "$WORK/ro.csv"
    The status should be success
    The stderr should include 'nothing to save'
  End

  It 'writes no directory export for an unchanged session on .save DIR'
    printf 'user_name,identifier\nalice,1' > "$WORK/ro2.csv"
    Data:expand
      #|SELECT 1;
      #|.save $WORK/out
    End
    When run sqly "$WORK/ro2.csv"
    The status should be success
    The output should include '1'
    The stderr should include 'nothing to save'
    The path "$WORK/out" should not be exist
  End
End
