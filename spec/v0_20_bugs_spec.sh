#!/bin/sh
# shellcheck shell=sh
#
# Binary-level regressions for the v0.20.0 hardening work. These run the built
# sqly the way a user does (flags, batch stdin, --sql-file, exit codes, stdout vs
# stderr) so the fixes are pinned at the layer the bugs were reported against.

Describe 'sqly v0.20.0 binary regressions'
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

  # --save / --save-dir reject schema-only and maintenance statements up front
  # instead of exiting 0 while leaving the source unchanged. Ref #433-#437,
  # #469-#484.
  Describe 'write-back rejects save-incompatible statements'
    Parameters
      "ALTER TABLE user RENAME COLUMN first_name TO fname" # 433
      "DROP TABLE user"                                    # 435
      "CREATE VIEW v AS SELECT user_name FROM user"        # 469
      "CREATE INDEX idx ON user(identifier)"               # 471
      "CREATE TABLE backup (id INTEGER)"                   # 473
      "REINDEX"                                            # 479
      "ANALYZE"                                            # 481
    End

    It "rejects: $1"
      When run sqly --sql "$1" --save --force "$WORK/user.csv"
      The status should be failure
      The stderr should be present
      The output should not include 'affected is'
    End
  End

  It 'rejects CREATE TABLE AS SELECT under --save-dir and writes nothing (#437)'
    When run sqly --sql "CREATE TABLE backup AS SELECT * FROM user" --save-dir "$WORK/out" "$WORK/user.csv"
    The status should be failure
    The stderr should be present
    The path "$WORK/out" should not be exist
  End

  It 'preflight rejects a CTAS+DML script before it executes (#438)'
    printf 'CREATE TABLE backup AS SELECT * FROM user;\nUPDATE user SET first_name=%cZ%c WHERE identifier=1;\n' "'" "'" > "$WORK/s.sql"
    When run sqly --sql-file "$WORK/s.sql" --save-dir "$WORK/out" "$WORK/user.csv"
    The status should be failure
    The stderr should be present
  End

  It 'allows a .import + UPDATE batch under --save-dir and writes the change (#456)'
    printf '.import testdata/user.csv\nUPDATE user SET first_name=%cBatch%c WHERE identifier=1;\n' "'" "'" > "$WORK/imp.sql"
    When run sqly --sql-file "$WORK/imp.sql" --save-dir "$WORK/out"
    The status should be success
    The output should include 'affected is 1 row(s)'
    The stderr should be present
    The contents of file "$WORK/out/user.csv" should include 'Batch'
  End

  # DDL / PRAGMA / maintenance no longer print a misleading affected-row count.
  Describe 'no-rowset statements report neutral success (#439)'
    Parameters
      "CREATE VIEW v AS SELECT 1 AS x"
      "CREATE TABLE t (id INTEGER)"
      "ANALYZE"
    End

    It "neutral success: $1"
      When run sqly --sql "$1"
      The status should be success
      The output should include 'statement executed successfully'
      The output should not include 'affected is'
    End
  End

  # Setter and command PRAGMAs run on the exec path instead of failing. Ref #440,
  # #485.
  It 'runs a setter PRAGMA (#440)'
    When run sqly --sql "PRAGMA user_version = 1" "$WORK/user.csv"
    The status should be success
    The output should include 'statement executed successfully'
  End

  It 'runs a command PRAGMA that returns no rows (#485)'
    When run sqly --sql "PRAGMA incremental_vacuum"
    The status should be success
    The output should include 'statement executed successfully'
  End

  # Transaction control, VACUUM, and ATTACH are rejected with a clear error.
  It 'rejects BEGIN in a --sql-file script (#441)'
    printf 'BEGIN IMMEDIATE;\nUPDATE user SET first_name=%cTX%c WHERE identifier=1;\nCOMMIT;\n' "'" "'" > "$WORK/tx.sql"
    When run sqly --sql-file "$WORK/tx.sql" "$WORK/user.csv"
    The status should be failure
    The stderr should include 'transaction'
  End

  It 'rejects VACUUM (#457)'
    When run sqly --sql "VACUUM"
    The status should be failure
    The stderr should include 'VACUUM'
  End

  It 'rejects VACUUM INTO and writes no file (#458)'
    When run sqly --sql "VACUUM INTO '$WORK/dump.db'"
    The status should be failure
    The stderr should include 'VACUUM'
    The path "$WORK/dump.db" should not be exist
  End

  It 'rejects ATTACH DATABASE and persists no external file (#443)'
    printf "ATTACH DATABASE '%s/aux.db' AS aux;\nCREATE TABLE aux.t (id INTEGER);\n" "$WORK" > "$WORK/a.sql"
    When run sqly --sql-file "$WORK/a.sql"
    The status should be failure
    The stderr should be present
    The path "$WORK/aux.db" should not be exist
  End

  # A multiline CREATE TRIGGER is parsed as one statement. Ref #468.
  It 'runs a multiline CREATE TRIGGER from a --sql-file (#468)'
    printf 'CREATE TRIGGER trig_user AFTER UPDATE ON user BEGIN\n  UPDATE user SET last_name=%cTriggered%c WHERE identifier=2;\nEND;\n' "'" "'" > "$WORK/t.sql"
    When run sqly --sql-file "$WORK/t.sql" "$WORK/user.csv"
    The status should be success
    The output should include 'statement executed successfully'
  End

  # Helper command surface: schema-qualified names, views, and TEMP tables.
  It 'accepts a schema-qualified .schema name (#445)'
    Data
      #|.import testdata/user.csv
      #|.schema main.user
    End
    When run sqly
    The status should be success
    The output should include 'user_name'
  End

  It 'lists session-created views and temp tables in .tables (#449, #450)'
    Data
      #|CREATE TEMP TABLE temp_t (id INTEGER);
      #|CREATE VIEW v_user AS SELECT user_name FROM user;
      #|.tables
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The output should include 'temp_t'
    The output should include 'v_user'
  End

  It 'prints CREATE VIEW for a view in .schema (#451)'
    Data
      #|CREATE VIEW v_user AS SELECT user_name FROM user;
      #|.schema v_user
    End
    When run sqly "$WORK/user.csv"
    The status should be success
    The output should include 'CREATE VIEW'
  End

  # Empty compressed JSON/JSONL import as zero-row tables. Ref #452, #453.
  It 'imports an empty compressed JSON array as a zero-row table (#452)'
    printf '[]' > "$WORK/empty.json"
    gzip -c "$WORK/empty.json" > "$WORK/empty.json.gz"
    When run sqly --sql "SELECT COUNT(*) AS c FROM empty" "$WORK/empty.json.gz"
    The status should be success
    The output should include '0'
  End

  It 'imports an empty compressed JSONL file as a zero-row table (#453)'
    : > "$WORK/empty.jsonl"
    gzip -c "$WORK/empty.jsonl" > "$WORK/empty.jsonl.gz"
    When run sqly --sql "SELECT COUNT(*) AS c FROM empty" "$WORK/empty.jsonl.gz"
    The status should be success
    The output should include '0'
  End

  # Standard Unix pseudo-files import end-to-end (staged as CSV). Ref #461, #462.
  It 'imports /dev/stdin as CSV (#461)'
    Data
      #|name,score
      #|a,1
      #|b,2
      #|c,3
    End
    When run sqly --csv --sql "SELECT COUNT(*) AS c FROM stdin" /dev/stdin
    The status should be success
    The line 2 should equal '3'
  End

  It 'imports /proc/self/fd/0 as CSV (#462)'
    if [ ! -e /proc/self/fd/0 ]; then
      Skip '/proc not available'
    fi
    Data
      #|name,score
      #|a,1
      #|b,2
      #|c,3
    End
    When run sqly --csv --sql "SELECT COUNT(*) AS c FROM sheet_0" /proc/self/fd/0
    The status should be success
    The line 2 should equal '3'
  End

  # ACH/Fedwire destinations hidden behind stacked compression suffixes are
  # rejected. Ref #459, #460.
  It 'rejects --output to a multi-compressed ACH destination (#459)'
    When run sqly --sql "SELECT * FROM user LIMIT 1" --output "$WORK/out.ach.gz.zst" "$WORK/user.csv"
    The status should be failure
    The stderr should include 'ACH/Fedwire'
    The path "$WORK/out.ach.gz.zst" should not be exist
  End

  It 'rejects .dump to a multi-compressed Fedwire destination (#460)'
    Data:expand
      #|.import testdata/user.csv
      #|.dump user ${WORK}/out.fed.gz.zst
    End
    When run sqly
    The status should be failure
    The stderr should include 'ACH/Fedwire'
    The path "$WORK/out.fed.gz.zst" should not be exist
  End

  # LTSV output and import reject invalid or duplicate labels. Ref #465, #466,
  # #467.
  It 'rejects an invalid LTSV output label (#465)'
    When run sqly --ltsv --sql 'SELECT 1 AS "foo:bar"' --output "$WORK/out.ltsv"
    The status should be failure
    The stderr should be present
  End

  It 'rejects duplicate LTSV output labels (#466)'
    When run sqly --ltsv --sql 'SELECT 1 AS x, 2 AS x' --output "$WORK/out.ltsv"
    The status should be failure
    The stderr should be present
  End

  It 'rejects an LTSV import with duplicate labels (#467)'
    printf 'x:1\tx:2\n' > "$WORK/dup.ltsv"
    When run sqly --sql 'SELECT * FROM dup' "$WORK/dup.ltsv"
    The status should be failure
    The stderr should be present
  End
End
