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

  # A helper command missing a required argument must fail the batch run (non-zero
  # exit) instead of printing usage and continuing as if it succeeded.
  It 'fails batch mode when .schema is missing its table name'
    Data
      #|.schema
      #|SELECT 1;
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.schema requires'
  End

  It 'fails batch mode when .header is missing its table name'
    Data
      #|.header
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.header requires'
  End

  It 'fails batch mode when .describe is missing its table name'
    Data
      #|.describe
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.describe requires'
  End

  It 'fails batch mode when .mode is missing its mode name'
    Data
      #|.mode
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.mode requires'
  End

  It 'fails batch mode when .dump is missing its destination'
    Data
      #|.dump
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.dump requires'
  End

  It 'fails batch mode when .import is missing its path'
    Data
      #|.import
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.import requires'
  End

  It 'fails batch mode when .save is missing its argument'
    Data
      #|.save
    End
    When run sqly "$PROJECT_ROOT/testdata/user.csv"
    The status should be failure
    The stderr should include '.save requires'
  End

  # Report-only modes must stay quiet on stderr after a successful directory import,
  # so the structured report is the only noteworthy output of a clean run.
  It 'keeps --inspect quiet on stderr after a successful directory import'
    When run sqly --inspect "$PROJECT_ROOT/testdata/space dir"
    The status should be success
    The output should include '"tables"'
    The stderr should not include 'Successfully imported'
  End

  It 'keeps --profile quiet on stderr after a successful directory import'
    When run sqly --profile "$PROJECT_ROOT/testdata/space dir"
    The status should be success
    The output should include '"tables"'
    The stderr should not include 'Successfully imported'
  End
End
