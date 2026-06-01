#!/bin/sh
# shellcheck shell=sh
#
# --output requires --sql. --output is only honored by the --sql
# path, so without --sql (no query or batch stdin) it must be rejected instead
# of silently ignored.

Describe 'sqly --output requires --sql'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    OUT_DIR=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$OUT_DIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'rejects --output with no query'
    Data
      #|
    End
    When run sqly testdata/user.csv --output "$OUT_DIR/out.csv"
    The status should be failure
    The stderr should include '--output requires --sql'
    The path "$OUT_DIR/out.csv" should not be exist
  End

  It 'rejects --output for batch SQL from stdin'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv --output "$OUT_DIR/out.csv"
    The status should be failure
    The stderr should include '--output requires --sql'
    The path "$OUT_DIR/out.csv" should not be exist
  End
End
