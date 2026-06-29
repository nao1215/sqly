#!/bin/sh
# shellcheck shell=sh
#
# --sheet validation end-to-end tests. --sheet only affects Excel
# imports, so it must be rejected for non-Excel inputs instead of being
# silently ignored.

Describe 'sqly --sheet validation'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'rejects --sheet with a non-Excel file and --sql'
    When run sqly --sql "SELECT * FROM user" --sheet "A test" testdata/user.csv
    The status should be failure
    The stderr should include '--sheet'
  End

  It 'rejects --sheet with a non-Excel file and --inspect'
    When run sqly --inspect --sheet "A test" testdata/user.csv
    The status should be failure
    The stderr should include '--sheet'
  End

  It 'still imports an Excel file with --sheet'
    When run sqly --csv --sql "SELECT * FROM sample_test_sheet" --sheet test_sheet testdata/sample.xlsx
    The status should be success
    The output should include 'name'
  End

  It 'rejects an explicit empty --sheet'
    When run sqly --inspect --sheet "" testdata/user.csv
    The status should be failure
    The stderr should include 'sheet'
  End

  It 'rejects --sheet for a directory with no Excel files'
    work=$(mktemp -d)
    cp testdata/user.csv "$work/u.csv"
    When run sqly --inspect --sheet anything "$work"
    The status should be failure
    The stderr should include '--sheet'
    rm -rf "$work"
  End

  It 'tells the user how to recover when --sheet has no Excel input'
    When run sqly --inspect --sheet "A test" testdata/user.csv
    The status should be failure
    The stderr should include 'Excel'
    The stderr should include 'remove --sheet'
  End

  It 'names the workbook and suggests recovery on a single-workbook sheet miss'
    When run sqly --inspect --sheet no_such_sheet testdata/sample.xlsx
    The status should be failure
    The stderr should include 'sample.xlsx'
    The stderr should include 'without --sheet'
  End

  It 'names every checked workbook on a multi-workbook sheet miss'
    When run sqly --inspect --sheet no_such_sheet testdata/sample.xlsx testdata/sheet_with_accents.xlsx
    The status should be failure
    The stderr should include 'sample.xlsx'
    The stderr should include 'sheet_with_accents.xlsx'
    The stderr should include 'without --sheet'
  End
End
