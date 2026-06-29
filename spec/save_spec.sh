#!/bin/sh
# shellcheck shell=sh
#
# Write-back end-to-end tests. DML changes are persisted to files only
# through the explicit --save / --save-dir flags and the .save command. --save
# overwrites sources and requires --force; --save-dir never touches the source.

Describe 'sqly write-back'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    export WORK
    cp testdata/user.csv "$WORK/u.csv"
  }
  cleanup() {
    rm -rf "${WORK:-}"
  }
  Before 'setup'
  After 'cleanup'

  It 'writes to --save-dir without modifying the source'
    When run sqly --sql "UPDATE u SET first_name = 'CHANGED' WHERE identifier = 1" "$WORK/u.csv" --save-dir "$WORK/out"
    The status should be success
    The output should include 'affected'
    The stderr should include 'Saved u to'
    The contents of file "$WORK/u.csv" should not include 'CHANGED'
    The contents of file "$WORK/out/u.csv" should include 'CHANGED'
  End

  It 'refuses --save without --force'
    When run sqly --sql "UPDATE u SET first_name = 'X'" "$WORK/u.csv" --save
    The status should be failure
    The stderr should include '--force'
    The contents of file "$WORK/u.csv" should not include 'X,'
  End

  It 'overwrites the source in place with --save --force'
    When run sqly --sql "DELETE FROM u WHERE identifier > 1" "$WORK/u.csv" --save --force
    The status should be success
    The output should include 'affected'
    The stderr should include 'Saved u to'
    # Re-import the rewritten file: only one row remains.
    rm -rf "$WORK/out"
  End

  It 're-imports a file rewritten in place (round-trip)'
    sqly --sql "DELETE FROM u WHERE identifier > 1" "$WORK/u.csv" --save --force >/dev/null 2>&1
    When run sqly --csv --sql "SELECT COUNT(*) AS c FROM u" "$WORK/u.csv"
    The status should be success
    The line 2 should equal '1'
  End

  It 'preserves gzip compression on in-place save'
    gzip -c testdata/user.csv > "$WORK/c.csv.gz"
    sqly --sql "UPDATE c SET first_name = 'GZ' WHERE identifier = 1" "$WORK/c.csv.gz" --save --force >/dev/null 2>&1
    When run sqly --csv --sql "SELECT first_name FROM c WHERE identifier = 1" "$WORK/c.csv.gz"
    The status should be success
    The line 2 should equal 'GZ'
  End

  It 'saves via the .save command in batch mode'
    Data
      #|UPDATE u SET first_name = 'BATCH' WHERE identifier = 1;
      #|.save --force
    End
    When run sqly "$WORK/u.csv"
    The status should be success
    The output should include 'affected'
    The stderr should include 'Saved u to'
    The contents of file "$WORK/u.csv" should include 'BATCH'
  End

  It 'guides a non-interactive --save with no input files toward passing input'
    When run sqly --save --force --sql "UPDATE foo SET x = 1"
    The status should be failure
    The stderr should include 'no tables to save'
    The stderr should include 'input files'
  End

  It 'guides a batch .save with no imported tables toward passing input'
    Data
      #|.save --force
    End
    When run sqly
    The status should be failure
    The stderr should include 'no tables to save'
    The stderr should include 'input files'
  End
End
