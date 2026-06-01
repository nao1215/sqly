#!/bin/sh
# shellcheck shell=sh
#
# End-to-end regression tests for the v0.19.0 binary bugs (#380-#431). Each case
# runs the built binary the way a user does and asserts the fixed behavior.

Describe 'sqly v0.19.0 binary bug fixes'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'machine-readable output stays valid (#380-#385, #426)'
    It 'quotes a CSV value containing a comma (#380)'
      When run sqly --csv --sql "SELECT 'a,b' AS c"
      The status should be success
      The line 2 should equal '"a,b"'
    End

    It 'quotes a CSV value containing a double quote (#380)'
      When run sqly --csv --sql "SELECT 'a' || char(34) || 'b' AS c"
      The status should be success
      The line 2 should equal '"a""b"'
    End

    It 'rejects an LTSV value containing a tab (#382)'
      When run sqly --ltsv --sql "SELECT 'a' || char(9) || 'b' AS c"
      The status should be failure
      The stderr should include 'LTSV'
    End

    It 'rejects duplicate JSON keys (#384)'
      When run sqly --json --sql "SELECT 1 AS x, 2 AS x"
      The status should be failure
      The stderr should include 'unique column names'
    End

    It 'rejects duplicate NDJSON keys (#385)'
      When run sqly --ndjson --sql "SELECT 1 AS x, 2 AS x"
      The status should be failure
      The stderr should include 'unique column names'
    End

    It 'keeps a Markdown row on one line when a value has a newline (#426)'
      When run sqly --markdown --sql "SELECT 'a' || char(10) || 'b' AS x"
      The status should be success
      The output should include 'a<br>b'
    End
  End

  Describe 'direct --sql accepts more SQLite statements (#386-#411, #430, #431)'
    It 'accepts a leading block comment (#386)'
      When run sqly --csv --sql "/* note */ SELECT 1 AS x"
      The status should be success
      The line 2 should equal '1'
    End

    It 'accepts PRAGMA (#406)'
      When run sqly --csv --sql "PRAGMA table_info(user)" testdata/user.csv
      The status should be success
      The output should include 'user_name'
    End

    It 'accepts VALUES (#407)'
      When run sqly --csv --sql "VALUES (1), (2)"
      The status should be success
      The output should include '1'
    End

    It 'accepts the TABLE shorthand (#408)'
      When run sqly --csv --sql "TABLE user" testdata/user.csv
      The status should be success
      The output should include 'booker12'
    End

    It 'accepts CREATE TABLE (#411)'
      When run sqly --sql "CREATE TABLE t(x)"
      The status should be success
      The output should include 'affected is 0'
    End

    It 'accepts ANALYZE (#431)'
      When run sqly --sql "ANALYZE" testdata/user.csv
      The status should be success
      The output should include 'affected is'
    End

    It 'runs WITH ... UPDATE without RETURNING as DML (#412)'
      When run sqly --sql "WITH s AS (SELECT 1 AS identifier) UPDATE user SET first_name='Z' WHERE identifier IN (SELECT identifier FROM s)" testdata/user.csv
      The status should be success
      The output should include 'affected is 1 row'
    End
  End

  Describe 'dependent flags are validated (#391-#393)'
    It 'rejects --stdin-name without --stdin (#391)'
      When run sqly --stdin-name weird --csv --sql "SELECT 1 AS x"
      The status should be failure
      The stderr should include 'stdin-name'
    End

    It 'rejects --inspect-sample without --inspect (#392)'
      When run sqly --inspect-sample 0 --csv --sql "SELECT 1 AS x"
      The status should be failure
      The stderr should include 'inspect-sample'
    End

    It 'rejects --force without --save (#393)'
      When run sqly --force --sql "SELECT 1 AS x"
      The status should be failure
      The stderr should include 'force'
    End

    It 'rejects --inspect combined with --csv (#390)'
      When run sqly --inspect --csv testdata/user.csv
      The status should be failure
      The stderr should include 'inspect'
    End
  End

  Describe 'empty JSON inputs import as zero-row tables (#388, #389)'
    It 'imports an empty JSON array (#388)'
      empty_json="$(mktemp -d)/empty.json"
      printf '[]' > "$empty_json"
      When run sqly --csv --sql "SELECT COUNT(*) AS n FROM empty" "$empty_json"
      The status should be success
      The line 2 should equal '0'
    End

    It 'imports an empty JSONL file (#389)'
      empty_jsonl="$(mktemp -d)/empty.jsonl"
      : > "$empty_jsonl"
      When run sqly --csv --sql "SELECT COUNT(*) AS n FROM empty" "$empty_jsonl"
      The status should be success
      The line 2 should equal '0'
    End
  End

  Describe 'output destination safety (#419, #421)'
    It 'rejects an --output path ending with a slash (#419)'
      out="$(mktemp -d)/outdir/"
      When run sqly --sql "SELECT 1 AS x" --output "$out" testdata/user.csv
      The status should be failure
      The stderr should include 'separator'
    End

    It 'rejects an --output ACH destination (#421)'
      out="$(mktemp -d)/out.ach"
      When run sqly --sql "SELECT identifier FROM user LIMIT 1" --output "$out" testdata/user.csv
      The status should be failure
      The stderr should include 'input-only'
    End
  End

  Describe 'batch helper commands run line-by-line (#397, #425)'
    It 'parses a helper command after a terminated statement (#397)'
      Data
        #|SELECT 1 AS x;
        #|.mode csv
        #|SELECT 2 AS y;
      End
      When run sqly
      The status should be success
      The output should include '2'
      The stderr should not include 'arguments'
    End

    It 'parses a helper command after a leading comment (#425)'
      Data
        #|-- header
        #|.mode csv
        #|SELECT 1 AS x;
      End
      When run sqly
      The status should be success
      The output should include '1'
      The stderr should not include 'arguments'
    End
  End

  Describe 'write-back semantics (#402, #404)'
    setup_user() {
      WORK="$(mktemp -d)"
      cp "$PROJECT_ROOT/testdata/user.csv" "$WORK/user.csv"
    }

    It 'does not write back for an EXPLAIN under --save-dir (#402)'
      setup_user
      out="$WORK/out"
      When run sqly --sql "EXPLAIN UPDATE user SET first_name='X' WHERE identifier=1" --save-dir "$out" "$WORK/user.csv"
      The status should be success
      The output should be present
      The path "$out/user.csv" should not be exist
    End

    It 'does not write back for a zero-row DML under --save-dir (#404)'
      setup_user
      out="$WORK/out"
      When run sqly --sql "UPDATE user SET first_name='X' WHERE identifier=999999" --save-dir "$out" "$WORK/user.csv"
      The status should be success
      The output should include 'affected is 0'
      The path "$out/user.csv" should not be exist
    End

    It 'keeps stdout clean when parquet write-back fails (#396)'
      WORK="$(mktemp -d)"
      cp "$PROJECT_ROOT/testdata/products.parquet" "$WORK/products.parquet"
      When run sqly --sql "DELETE FROM products" --save --force "$WORK/products.parquet"
      The status should be failure
      The output should equal ''
      The stderr should be present
    End
  End

  Describe 'input path validation (#427)'
    It 'accepts a user file under /dev/shm (#427)'
      Skip if "no /dev/shm" [ ! -d /dev/shm ]
      shm="$(mktemp -d /dev/shm/sqly-XXXXXX)"
      printf 'id,name\n1,a\n' > "$shm/data.csv"
      When run sqly --csv --sql "SELECT COUNT(*) AS n FROM data" "$shm/data.csv"
      The status should be success
      The line 2 should equal '1'
    End
  End
End
