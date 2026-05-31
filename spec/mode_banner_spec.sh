#!/bin/sh
# shellcheck shell=sh
#
# .mode change banner routing (#295). In batch mode the mode-change banner must
# not pollute stdout, so JSON and NDJSON output stay machine-readable. The
# banner is a status message and goes to stderr instead.

Describe 'sqly .mode banner routing (#295)'
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
End
