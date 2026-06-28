#!/bin/sh
# shellcheck shell=sh
#
# Input path validation false positives. sqly runs locally with the
# user's own permissions, so legitimate readable paths must import regardless of
# nesting depth or filenames that merely contain traversal-looking byte
# sequences.

Describe 'sqly input path validation'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'imports a deeply nested path'
    deep="$WORK/a/b/c/d/e/f/g/h/i/j/k"
    mkdir -p "$deep"
    cp testdata/user.csv "$deep/user.csv"
    When run sqly --csv --sql "SELECT COUNT(*) AS c FROM user" "$deep/user.csv"
    The status should be success
    The line 2 should equal '3'
  End

  It 'imports a file whose name literally contains ..%2f'
    cp testdata/user.csv "$WORK/..%2fuser.csv"
    When run sqly --inspect "$WORK/..%2fuser.csv"
    The status should be success
    The output should include 'user_name'
    The stderr should not include 'dangerous path pattern'
  End

  # A symlink alias to a blocked system file must be rejected just like the
  # direct path; otherwise the guard only checks the typed string, not the real
  # target.
  It 'rejects a symlink alias that resolves to a blocked system path'
    Skip if 'no /etc/hosts on this host' test ! -e /etc/hosts
    ln -sf /etc/hosts "$WORK/hosts_alias.csv"
    When run sqly --inspect "$WORK/hosts_alias.csv"
    The status should be failure
    The stderr should include 'system directory not allowed'
  End

  It 'imports a symlink alias that resolves to an ordinary user file'
    cp testdata/user.csv "$WORK/real.csv"
    ln -sf "$WORK/real.csv" "$WORK/user_alias.csv"
    When run sqly --inspect "$WORK/user_alias.csv"
    The status should be success
    The output should include 'user_name'
  End
End
