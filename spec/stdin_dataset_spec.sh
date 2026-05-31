#!/bin/sh
# shellcheck shell=sh
#
# stdin-as-dataset end-to-end tests (#258). With --stdin <format>, piped stdin
# is imported as a dataset (default table "stdin") instead of being read as
# SQL/helper commands, and it can be joined with file arguments. Without
# --stdin, batch mode behavior is unchanged.

Describe 'sqly --stdin dataset'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'querying piped data'
    It 'queries piped CSV through the default stdin table'
      Data
        #|id,name
        #|1,alice
        #|2,bob
      End
      When run sqly --stdin csv --csv --sql "SELECT name FROM stdin ORDER BY id"
      The status should be success
      The line 1 should equal 'name'
      The line 2 should equal 'alice'
      The line 3 should equal 'bob'
    End

    It 'queries piped TSV data'
      Data
        #|id	name
        #|1	alice
      End
      When run sqly --stdin tsv --csv --sql "SELECT COUNT(*) AS c FROM stdin"
      The status should be success
      The output should include '1'
    End

    It 'overrides the stdin table name with --stdin-name'
      Data
        #|id,name
        #|1,alice
        #|2,bob
      End
      When run sqly --stdin csv --stdin-name people --csv --sql "SELECT COUNT(*) FROM people"
      The status should be success
      The output should include '2'
    End
  End

  Describe 'joining stdin with a file'
    It 'joins piped stdin with an imported file argument'
      Data
        #|id,name
        #|1,alice
        #|2,bob
      End
      When run sqly --stdin csv --csv --sql "SELECT s.name, i.position FROM stdin s JOIN identifier i ON s.id = i.id ORDER BY s.id" testdata/identifier.csv
      The status should be success
      The output should include 'alice'
      The output should include 'developrt'
    End
  End

  Describe 'error handling'
    It 'reports a clear error for an unsupported stdin format'
      Data
        #|a,b
        #|1,2
      End
      When run sqly --stdin xml --sql "SELECT 1"
      The status should be failure
      The stderr should include 'unsupported --stdin format'
    End
  End

  Describe 'batch mode is unchanged without --stdin'
    It 'still reads stdin as SQL and helper commands'
      Data
        #|.tables
        #|SELECT user_name FROM user ORDER BY identifier LIMIT 1
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'TABLE NAME'
      The output should include 'booker12'
    End
  End
End
