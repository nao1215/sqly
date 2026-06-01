#!/bin/sh
# shellcheck shell=sh
#
# History-storage tolerance end-to-end tests. Non-interactive runs must
# succeed even when the history database cannot be created or written. The
# history DB path is pointed at a directory that does not exist, so creating it
# fails the way a read-only or sandboxed config location would.

Describe 'sqly history tolerance'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup_unwritable_history() {
    HIST_DIR=$(mktemp -d)
    export HIST_DIR
    # The "missing" parent does not exist, so the history DB cannot be created.
    SQLY_HISTORY_DB_PATH="$HIST_DIR/missing/history.db"
    export SQLY_HISTORY_DB_PATH
  }
  cleanup_unwritable_history() {
    rm -rf "${HIST_DIR:-}"
    unset SQLY_HISTORY_DB_PATH
  }
  Before 'setup_unwritable_history'
  After 'cleanup_unwritable_history'

  It 'runs --sql and warns instead of failing'
    When run sqly --csv --sql "SELECT actor FROM actor ORDER BY actor LIMIT 1" testdata/actor.csv
    The status should be success
    The output should include 'Adam Sandler'
    The stderr should include 'history disabled'
  End

  It 'runs batch mode (.tables plus a query) without a writable history DB'
    Data
      #|.tables
      #|SELECT actor FROM actor ORDER BY actor LIMIT 1
    End
    When run sqly testdata/actor.csv
    The status should be success
    The output should include 'TABLE NAME'
    The output should include 'Adam Sandler'
    The stderr should include 'history disabled'
  End
End

# Runtime history failures: the DB is created successfully at startup but a
# later write fails, simulating a history DB that becomes read-only mid-session.
Describe 'sqly history tolerance after startup'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup_readonly_history() {
    RO_DIR=$(mktemp -d)
    export RO_DIR
    RO_HIST="$RO_DIR/history.db"
    # Seed a valid history DB so the table exists, then make the file reject
    # writes. Startup's CREATE TABLE IF NOT EXISTS is then a no-op that succeeds,
    # while a later history insert fails like a DB that turned read-only.
    printf 'SELECT 1\n' | SQLY_HISTORY_DB_PATH="$RO_HIST" "$SQLY_BIN" "$PROJECT_ROOT/testdata/user.csv" >/dev/null
    chmod 0444 "$RO_HIST"
    SQLY_HISTORY_DB_PATH="$RO_HIST"
    export SQLY_HISTORY_DB_PATH
  }
  cleanup_readonly_history() {
    rm -rf "${RO_DIR:-}"
    unset SQLY_HISTORY_DB_PATH
  }
  Before 'setup_readonly_history'
  After 'cleanup_readonly_history'

  It 'runs --sql when the history DB is read-only after startup'
    When run sqly --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv
    The status should be success
    The output should include 'booker12'
  End

  It 'runs --inspect when the history DB is read-only after startup'
    When run sqly --inspect testdata/user.csv
    The status should be success
    The line 1 should equal '{'
    The output should include '"name": "user"'
  End

  It 'runs batch mode and warns when a history write fails after startup'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should include 'booker12'
    The stderr should include 'history disabled'
  End
End
