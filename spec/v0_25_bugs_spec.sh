#!/bin/sh
# shellcheck shell=sh
#
# Binary-level regressions for the v0.25.0 onboarding/UX hardening work. These run
# the built sqly the way a user does (flags, batch stdin, exit codes) so the fixes
# are pinned at the layer the bugs were reported against.

Describe 'sqly v0.25.0 binary regressions'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    export SQLY_HISTORY_DB_PATH="$WORK/history.db"
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  # An explicit empty --sql value must fail fast instead of silently running no
  # query and exiting 0, matching the other string flags that reject empty values.
  It 'rejects an explicit empty --sql value'
    When run sqly --sql '' "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '--sql requires a non-empty SQL statement'
  End

  # A non-interactive run that receives no statements (no TTY, empty stdin, no
  # --sql/--sql-file) must surface a hint and exit non-zero instead of looking
  # successful while doing nothing.
  It 'reports a hint when non-interactive run gets empty stdin and no file'
    When run sh -c "printf '' | \"$SQLY_BIN\""
    The status should be failure
    The stderr should include 'no TTY detected'
  End

  It 'reports a hint when non-interactive run gets empty stdin with a file'
    When run sh -c "printf '' | \"$SQLY_BIN\" \"$PROJECT_ROOT/testdata/user.csv\""
    The status should be failure
    The stderr should include 'no TTY detected'
  End

  # A failing --stdin import must describe the input as stdin, not leak the random
  # internal staging temp path (which is noisy and changes every run).
  It 'reports a stable stdin reference instead of the staging temp path'
    When run sh -c "printf '' | \"$SQLY_BIN\" --stdin csv --sql 'SELECT COUNT(*) FROM stdin'"
    The status should be failure
    The stderr should include 'stdin'
    The stderr should not include 'sqly-stdin-'
  End
End
