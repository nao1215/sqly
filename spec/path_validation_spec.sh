#!/bin/sh
# shellcheck shell=sh
#
# Input path validation false positives (#316, #317). sqly runs locally with the
# user's own permissions, so legitimate readable paths must import regardless of
# nesting depth or filenames that merely contain traversal-looking byte
# sequences.

Describe 'sqly input path validation (#316, #317)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'imports a deeply nested path (#316)'
    deep="$WORK/a/b/c/d/e/f/g/h/i/j/k"
    mkdir -p "$deep"
    cp testdata/user.csv "$deep/user.csv"
    When run sqly --csv --sql "SELECT COUNT(*) AS c FROM user" "$deep/user.csv"
    The status should be success
    The line 2 should equal '3'
  End

  It 'imports a file whose name literally contains ..%2f (#317)'
    cp testdata/user.csv "$WORK/..%2fuser.csv"
    When run sqly --inspect "$WORK/..%2fuser.csv"
    The status should be success
    The output should include 'user_name'
    The stderr should not include 'dangerous path pattern'
  End
End
