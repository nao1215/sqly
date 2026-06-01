#!/bin/sh
# shellcheck shell=sh
#
# File-output status routing. When data is written to a file via
# --output, .dump, or .save, the success/status line is control-plane output
# and must go to stderr so stdout stays empty for scripts.

Describe 'sqly file-output status routing'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    OUT_DIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$OUT_DIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'keeps stdout empty for --output and reports on stderr'
    When run sqly --sql "SELECT 1 AS x" --output "$OUT_DIR/out.csv" testdata/user.csv
    The status should be success
    The output should equal ''
    The stderr should include 'Output sql result to'
    The path "$OUT_DIR/out.csv" should be file
  End

  It 'keeps stdout free of the .dump status line'
    Data:expand
      #|.dump user ${OUT_DIR}/dump.csv
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should equal ''
    The stderr should include 'dump `user` table to'
    The path "$OUT_DIR/dump.csv" should be file
  End

  It 'keeps the .save confirmation off stdout'
    cp testdata/user.csv "$OUT_DIR/u.csv"
    Data:expand
      #|UPDATE u SET first_name = 'X' WHERE identifier = 1;
      #|.save ${OUT_DIR}/saved
    End
    When run sqly "$OUT_DIR/u.csv"
    The status should be success
    The output should include 'affected'
    The output should not include 'Saved'
    The stderr should include 'Saved u to'
  End
End
