#!/bin/sh
# shellcheck shell=sh
#
# Proves the E2E suite runs in the isolated sandbox prepared by
# scripts/run_e2e.sh, so it never depends on the developer's real config
# directory. When the suite is run directly (not through the wrapper),
# SQLY_E2E_SANDBOX is unset and these checks skip instead of failing.

Describe 'hermetic E2E environment'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'runs with HOME inside the sandbox'
    Skip if 'not run through scripts/run_e2e.sh' test -z "${SQLY_E2E_SANDBOX:-}"
    When run sh -c 'case "$HOME" in "$SQLY_E2E_SANDBOX"*) exit 0 ;; *) exit 1 ;; esac'
    The status should be success
  End

  It 'routes the history DB into the sandbox'
    Skip if 'not run through scripts/run_e2e.sh' test -z "${SQLY_E2E_SANDBOX:-}"
    When run sh -c 'case "$SQLY_HISTORY_DB_PATH" in "$SQLY_E2E_SANDBOX"*) exit 0 ;; *) exit 1 ;; esac'
    The status should be success
  End

  It 'still runs sqly normally inside the sandbox'
    When run sqly --sql 'SELECT 1 AS one' "$PROJECT_ROOT/testdata/user.csv"
    The status should be success
    The output should include 'one'
  End
End
