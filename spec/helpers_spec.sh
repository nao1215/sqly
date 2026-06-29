#!/bin/sh
# shellcheck shell=sh
#
# Shell helper-command end-to-end tests: in-process .ls/.clear/.cd,
# quoted .import arguments, and the .mode listing. Driven through
# piped stdin so the real binary is exercised.

Describe 'sqly shell helper commands'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '.help'
    It 'groups commands, shows usage, and flags destructive save'
      Data
        #|.help
      End
      When run sqly
      The status should be success
      The output should include 'Import / Export'
      The output should include '.import PATH'
      The output should include '.dump TABLE FILE'
      The output should include '.save DIR'
      The output should include '.save --force'
      The output should include 'destructive'
    End
  End

  Describe '.cd and .pwd'
    It 'changes directory with a relative path and reports it'
      Data
        #|.cd testdata
        #|.pwd
      End
      When run sqly
      The status should be success
      The output should include 'testdata'
    End
  End

  Describe '~ home-directory expansion'
    # Point HOME at a throwaway directory so the expansion is verifiable
    # without touching the real home. A CSV lives inside it for .import.
    setup() {
      TILDE_HOME="$(mktemp -d)"
      export TILDE_HOME
      export HOME="$TILDE_HOME"
      printf 'id,name\n1,foo\n' > "$TILDE_HOME/sqly_tilde.csv"
    }
    cleanup() { rm -rf "${TILDE_HOME:-}"; }
    Before 'setup'
    After 'cleanup'

    It 'expands a bare ~ in .cd to the home directory'
      Data
        #|.cd ~
        #|.pwd
      End
      When run sqly
      The status should be success
      The output should include "$(basename "$TILDE_HOME")"
    End

    It 'expands ~/file in .import'
      Data
        #|.import ~/sqly_tilde.csv
        #|.tables
      End
      When run sqly
      The status should be success
      The output should include 'sqly_tilde'
    End
  End

  Describe '.clear'
    It 'emits no ANSI escapes to stdout in batch mode'
      # Piped stdin is non-TTY batch mode, where stdout may carry
      # machine-readable payloads. .clear must stay silent there.
      Data
        #|.clear
      End
      When run sqly
      The status should be success
      The output should equal ''
    End

    It 'keeps --json stdout parseable when .clear precedes a query'
      Data
        #|.clear
        #|SELECT 1 AS x;
      End
      When run sqly --json testdata/user.csv
      The status should be success
      The line 1 should equal '['
    End
  End

  Describe '.import with a quoted path containing a space'
    # Use a unique directory so concurrent runs cannot collide; the spaced
    # filename inside it is what the quoting must handle.
    setup() {
      IMPORT_DIR="$(mktemp -d)"
      export IMPORT_DIR
      printf 'id,name\n1,foo\n' > "$IMPORT_DIR/sqly_e2e space.csv"
    }
    cleanup() { rm -rf "${IMPORT_DIR:-}"; }
    Before 'setup'
    After 'cleanup'

    It 'imports the file as a single argument'
      Data:expand
        #|.import "$IMPORT_DIR/sqly_e2e space.csv"
        #|.tables
      End
      When run sqly
      The status should be success
      The output should include 'sqly_e2e_space'
    End
  End

  Describe '.mode'
    It 'fails on a missing mode name but still lists the modes'
      # A missing mode name is a command error so a batch script fails fast, but
      # the error still carries the mode list on stderr for discovery.
      Data
        #|.mode
      End
      When run sqly testdata/user.csv
      The status should be failure
      The stderr should include 'json'
      The stderr should include 'ndjson'
    End
  End

  Describe '.dump'
    It 'infers TSV from the .tsv extension in table mode, not CSV'
      # In table mode .dump picks the format from the destination extension, so
      # ".dump user out.tsv" writes a tab-separated file rather than falling back
      # to CSV. The header line must be tab-separated, not comma-separated.
      When run sh -c "out=\$(mktemp -d)/out.tsv; printf '.dump user %s\n' \"\$out\" | '$SQLY_BIN' '$PROJECT_ROOT/testdata/user.csv' >/dev/null; head -n1 \"\$out\""
      The status should be success
      The output should include "$(printf 'user_name\tidentifier')"
      The output should not include 'user_name,identifier'
      # The dump progress line confirms the inferred format on stderr.
      The stderr should include 'mode=tsv'
    End
  End
End
