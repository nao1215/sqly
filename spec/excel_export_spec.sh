#!/bin/sh
# shellcheck shell=sh
#
# Excel export end-to-end tests (#296). Exported .xlsx files must be ordinary
# data files, not executables, matching other output formats.

Describe 'sqly excel export (#296)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    OUT_DIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$OUT_DIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'writes a non-executable .xlsx with --output'
    When run sqly --excel --output "$OUT_DIR/out.xlsx" --sql "SELECT * FROM user LIMIT 1" testdata/user.csv
    The status should be success
    The stderr should include 'output mode=excel'
    The path "$OUT_DIR/out.xlsx" should be file
    The path "$OUT_DIR/out.xlsx" should not be executable
  End

  It 'writes a non-executable .xlsx with the .dump command'
    Data:expand
      #|.mode excel
      #|.dump user "$OUT_DIR/dump.xlsx"
    End
    When run sqly testdata/user.csv
    The status should be success
    The stderr should include 'mode=excel'
    The path "$OUT_DIR/dump.xlsx" should be file
    The path "$OUT_DIR/dump.xlsx" should not be executable
  End
End
