#!/bin/sh
# shellcheck shell=sh
#
# --inspect conflicting-flag validation. --inspect is self-contained, so
# combining it with action or side-effecting flags must fail instead of
# silently discarding them.

Describe 'sqly --inspect conflicts'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    OUT_DIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$OUT_DIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'rejects --inspect with --sql'
    When run sqly --inspect --sql "SELECT * FROM user LIMIT 1" testdata/user.csv
    The status should be failure
    The stderr should include '--inspect'
  End

  It 'rejects --inspect with --output and writes no file'
    When run sqly --inspect --output "$OUT_DIR/out.csv" testdata/user.csv
    The status should be failure
    The stderr should include '--inspect'
    The path "$OUT_DIR/out.csv" should not be exist
  End

  It 'rejects --inspect with --save-dir and creates no save dir'
    When run sqly --inspect --save-dir "$OUT_DIR/save" testdata/user.csv
    The status should be failure
    The stderr should include '--inspect'
    The path "$OUT_DIR/save" should not be exist
  End
End
