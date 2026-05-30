#!/bin/sh
# shellcheck shell=sh
#
# Non-TTY batch-mode end-to-end tests (#246). Piping into sqly (no terminal)
# runs commands from stdin; failures must surface a non-zero exit code.

Describe 'sqly batch mode (piped stdin)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'runs SQL read from stdin'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should include 'booker12'
  End

  It 'switches output mode and runs the following query'
    Data
      #|.mode ndjson
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should include '{"user_name":"booker12"}'
  End

  It 'exits non-zero and names the failing line on error'
    Data
      #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
      #|SELECT * FROM no_such_table
    End
    When run sqly testdata/user.csv
    The status should be failure
    The output should include 'booker12'
    The stderr should include 'batch line 2 failed'
    The stderr should include 'no_such_table'
  End

  It 'stops at .exit with a success status'
    Data
      #|.exit
      #|SELECT * FROM no_such_table
    End
    When run sqly testdata/user.csv
    The status should be success
  End
End
