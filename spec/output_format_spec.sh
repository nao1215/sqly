#!/bin/sh
# shellcheck shell=sh
#
# Output-format end-to-end tests ( and existing formats). Runs the binary
# with --json/--ndjson/--csv and checks the rendered query results.

Describe 'sqly output formats'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '--json'
    It 'renders results as a JSON array'
      When run sqly --json --sql "SELECT user_name, identifier FROM user ORDER BY identifier LIMIT 2" testdata/user.csv
      The status should be success
      The line 1 should equal '['
      The output should include '{"user_name":"booker12","identifier":"1"}'
      The output should include '{"user_name":"jenkins46","identifier":"2"}'
    End

    It 'prints [] for an empty result'
      When run sqly --json --sql "SELECT user_name FROM user WHERE user_name = 'nobody'" testdata/user.csv
      The status should be success
      The output should equal '[]'
    End
  End

  Describe '--ndjson'
    It 'renders one JSON object per line'
      When run sqly --ndjson --sql "SELECT user_name, identifier FROM user ORDER BY identifier LIMIT 2" testdata/user.csv
      The status should be success
      The line 1 should equal '{"user_name":"booker12","identifier":"1"}'
      The line 2 should equal '{"user_name":"jenkins46","identifier":"2"}'
    End

    It 'prints nothing for an empty result'
      When run sqly --ndjson --sql "SELECT user_name FROM user WHERE user_name = 'nobody'" testdata/user.csv
      The status should be success
      The output should equal ''
    End
  End

  Describe '--csv'
    It 'renders header and rows as CSV'
      When run sqly --csv --sql "SELECT user_name, identifier FROM user ORDER BY identifier LIMIT 1" testdata/user.csv
      The status should be success
      The line 1 should equal 'user_name,identifier'
      The line 2 should equal 'booker12,1'
    End
  End
End
