#!/bin/sh
# shellcheck shell=sh
#
# --sheet validation end-to-end tests (#287). --sheet only affects Excel
# imports, so it must be rejected for non-Excel inputs instead of being
# silently ignored.

Describe 'sqly --sheet validation (#287)'
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
End
