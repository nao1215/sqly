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

  Describe '.clear'
    It 'clears the screen in-process without an external command'
      clear_seq=$(printf '\033[H\033[2J\033[3J')
      Data
        #|.clear
      End
      When run sqly
      The status should be success
      The output should equal "$clear_seq"
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
