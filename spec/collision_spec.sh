#!/bin/sh
# shellcheck shell=sh
#
# Sanitized table-name collisions. Two inputs that sanitize to the same
# table name must fail instead of one silently overwriting the other.

Describe 'sqly table-name collision'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    printf 'id,name\n1,A\n' > "$WORK/a-b.csv"
    printf 'id,name\n2,B\n' > "$WORK/a_b.csv"
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'fails when two inputs sanitize to the same table name'
    When run sqly --inspect "$WORK/a-b.csv" "$WORK/a_b.csv"
    The status should be failure
    The stderr should include 'collision'
  End
End
