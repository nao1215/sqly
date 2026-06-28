#!/bin/sh
# shellcheck shell=sh
#
# Shell helper-command end-to-end tests: in-process .ls/.clear/.cd,
# quoted .import arguments, and the .mode listing. Driven through
# piped stdin so the real binary is exercised.

Describe 'sqly shell helper commands'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

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
    It 'lists json and ndjson modes'
      Data
        #|.mode
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'json'
      The output should include 'ndjson'
    End
  End
End
