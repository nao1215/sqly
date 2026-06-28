#!/bin/sh
# shellcheck shell=sh
#
# .mode change banner routing. In batch mode the mode-change banner must
# not pollute stdout, so JSON and NDJSON output stay machine-readable. The
# banner is a status message and goes to stderr instead.

Describe 'sqly .mode banner routing'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'keeps stdout pure JSON after .mode json'
    Data
      #|.mode json
      #|SELECT user_name FROM user LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The line 1 should equal '['
    The output should not include 'Change output mode'
    The stderr should include 'Change output mode from table to json'
  End

  It 'keeps stdout pure NDJSON after .mode ndjson'
    Data
      #|.mode ndjson
      #|SELECT user_name FROM user LIMIT 1
    End
    When run sqly testdata/user.csv
    The status should be success
    The line 1 should equal '{"user_name":"booker12"}'
    The output should not include 'Change output mode'
    The stderr should include 'Change output mode from table to ndjson'
  End

  It 'reports the typed mode by name and emits typed output after .mode json-typed'
    Data
      #|.mode json-typed
      #|SELECT 7 AS n, 'x' AS s
    End
    When run sqly testdata/user.csv
    The status should be success
    The output should not include 'Change output mode'
    The stderr should include 'Change output mode from table to json-typed'
    The output should include '"n":7'
    The output should include '"s":"x"'
  End
End
