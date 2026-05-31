#!/bin/sh
# shellcheck shell=sh
#
# History-storage tolerance end-to-end tests (#262). Non-interactive runs must
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
