#!/bin/sh
# shellcheck shell=sh
#
# Binary end-to-end tests for the bugs reported against the published v0.18.0
# binary (#349-#378). These exercise the CLI the way a user does: flags, piped
# stdin, exit codes, and stdout/stderr separation.

Describe 'sqly v0.18.0 binary bug fixes'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'explicit empty flag values are rejected'
    It 'rejects an empty --output (#349)'
      When run sqly --sql "SELECT 1 AS x" --output ""
      The status should be failure
      The stderr should include '--output'
    End

    It 'rejects an empty --sql-file (#350)'
      When run sqly --sql-file ""
      The status should be failure
      The stderr should include '--sql-file'
    End

    It 'rejects an empty --save-dir (#352)'
      When run sqly --sql "SELECT 1" --save-dir "" testdata/user.csv
      The status should be failure
      The stderr should include '--save-dir'
    End

    It 'rejects an empty --stdin (#353)'
      Data
        #|id,name
        #|1,a
      End
      When run sqly --stdin "" --sql "SELECT 1 AS x"
      The status should be failure
      The stderr should include '--stdin'
    End
  End

  Describe 'output mode and DML validation'
    It 'rejects conflicting output mode flags (#365)'
      When run sqly --csv --json --sql "SELECT 1 AS x"
      The status should be failure
      The stderr should include 'conflicting'
    End

    It 'prints rows for a DML RETURNING statement (#363)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/u.csv"
      When run sqly --csv --sql "UPDATE u SET first_name='X' WHERE identifier=1 RETURNING identifier, first_name" "$work/u.csv"
      The status should be success
      The output should include 'X'
      The output should not include 'affected'
      rm -rf "$work"
    End

    It 'rejects --output for a non-rowset DML statement (#364)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/u.csv"
      When run sqly --sql "UPDATE u SET first_name='X' WHERE identifier=1" --output "$work/out.csv" "$work/u.csv"
      The status should be failure
      The stderr should include '--output'
      The path "$work/out.csv" should not be exist
      rm -rf "$work"
    End

    It 'exports RETURNING rows with --output (#368)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/u.csv"
      When run sqly --csv --sql "UPDATE u SET first_name='X' WHERE identifier=1 RETURNING identifier" --output "$work/out.csv" "$work/u.csv"
      The status should be success
      The path "$work/out.csv" should be exist
      The stderr should include 'Output sql result'
      rm -rf "$work"
    End
  End

  Describe 'sql-file and stdin routing'
    It 'rejects a comment-only --sql-file (#351)'
      work=$(mktemp -d)
      printf -- '-- header only\n/* block */\n' > "$work/q.sql"
      When run sqly --sql-file "$work/q.sql"
      The status should be failure
      The stderr should include 'no executable'
      rm -rf "$work"
    End

    It 'strips a UTF-8 BOM from a --sql-file script (#369)'
      work=$(mktemp -d)
      printf '\357\273\277SELECT 2 AS y;\n' > "$work/q.sql"
      When run sqly --csv --sql-file "$work/q.sql" testdata/user.csv
      The status should be success
      The output should include '2'
      rm -rf "$work"
    End

    It 'strips a UTF-8 BOM from batch stdin (#369)'
      Data
        #|﻿SELECT 7 AS z;
      End
      When run sqly --csv testdata/user.csv
      The status should be success
      The output should include '7'
    End

    It 'rejects non-empty piped stdin with --sql-file (#373)'
      work=$(mktemp -d)
      printf 'SELECT 1 AS x;\n' > "$work/q.sql"
      Data
        #|SELECT 999 AS y;
      End
      When run sqly --sql-file "$work/q.sql" testdata/user.csv
      The status should be failure
      The stderr should include 'stdin'
      rm -rf "$work"
    End

    It 'fails a --stdin dataset run with no query (#374)'
      Data
        #|id,name
        #|1,a
      End
      When run sqly --stdin csv
      The status should be failure
      The stderr should include '--stdin'
    End
  End

  Describe 'directory imports'
    It 'reports per-file provenance for a sanitized basename (#357)'
      work=$(mktemp -d)
      printf 'id,name\n1,a\n' > "$work/2023-data.csv"
      When run sqly --inspect "$work"
      The status should be success
      The output should include '2023-data.csv'
      The stderr should include 'Successfully imported'
      rm -rf "$work"
    End

    It 'rejects duplicate basenames from different subdirectories (#359)'
      work=$(mktemp -d)
      mkdir -p "$work/a" "$work/b"
      printf 'id,name\n1,alpha\n' > "$work/a/user.csv"
      printf 'id,name\n2,beta\n' > "$work/b/user.csv"
      When run sqly --inspect "$work"
      The status should be failure
      The stderr should include 'collision'
      rm -rf "$work"
    End

    It 'reports an overwrite when re-importing a directory (#361)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/user.csv"
      mkdir "$work/dir"
      printf 'user_name,identifier,first_name,last_name\nalt1,1,ALT,One\n' > "$work/dir/user.csv"
      # Drive the .import + query via a --sql-file so the directory path can be
      # interpolated (shellspec Data heredocs cannot interpolate variables).
      printf '.import %s\nSELECT user_name FROM user ORDER BY identifier;\n' "$work/dir" > "$work/cmds.sql"
      When run sqly --sql-file "$work/cmds.sql" "$work/user.csv"
      The status should be success
      The output should include 'alt1'
      The stderr should not include 'No supported files'
      rm -rf "$work"
    End
  End

  Describe 'write-back safety'
    It 'rejects --output that aliases an imported source (#371)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/user.csv"
      When run sqly --csv --sql "SELECT * FROM user WHERE identifier=1" --output "$work/user.csv" "$work/user.csv"
      The status should be failure
      The stderr should include '--output'
      rm -rf "$work"
    End

    It 'rejects --save-dir that resolves to the source directory (#370)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/user.csv"
      When run sqly --sql "UPDATE user SET first_name='P' WHERE identifier=1" --save-dir "$work" "$work/user.csv"
      The status should be failure
      The stderr should include 'source'
      rm -rf "$work"
    End

    It 'rejects a --save-dir destination that already exists (#372)'
      work=$(mktemp -d)
      mkdir -p "$work/src" "$work/out"
      cp testdata/user.csv "$work/src/user.csv"
      cp testdata/user.csv "$work/out/user.csv"
      When run sqly --sql "UPDATE user SET first_name='Q' WHERE identifier=1" --save-dir "$work/out" "$work/src/user.csv"
      The status should be failure
      The stderr should include 'already exists'
      rm -rf "$work"
    End

    It 'keeps stdout clean when write-back fails (#375)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/user.csv"
      cp testdata/sample.xlsx "$work/sample.xlsx"
      When run sqly --sql "UPDATE user SET first_name='X' WHERE identifier=1" --save-dir "$work/out" "$work/user.csv" "$work/sample.xlsx"
      The status should be failure
      The output should not include 'affected'
      The stderr should include 'cannot save'
      rm -rf "$work"
    End

    It 'skips write-back for a read-only query under --save --force (#376)'
      work=$(mktemp -d)
      cp testdata/user.csv "$work/user.csv"
      When run sqly --csv --sql "SELECT * FROM user WHERE identifier=1" --save --force "$work/user.csv"
      The status should be success
      The output should include 'booker12'
      The stderr should not include 'Saved'
      rm -rf "$work"
    End
  End

  Describe 'multi-workbook --sheet'
    It 'skips workbooks lacking the requested sheet (#378)'
      When run sqly --inspect --sheet "A test" testdata/sheet_with_spaces.xlsx testdata/sample.xlsx testdata/sheet_with_accents.xlsx
      The status should be success
      The output should include 'sheet_with_spaces'
      The stderr should include 'Skipped'
    End
  End
End
