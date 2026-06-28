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
    The stderr should include '--sql'
  End
End
