#!/bin/sh
# shellcheck shell=sh
#
# Import-failure handling (#297, #300, #302, #306). When an explicitly requested
# input fails to import, non-interactive runs exit non-zero and keep import
# diagnostics on stderr so stdout stays machine-readable.

Describe 'sqly import failure handling'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'fails query mode on a partial import and keeps stdout clean (#297, #306)'
    When run sqly --json --sql "SELECT user_name FROM user LIMIT 1" testdata/user.csv /no/such/file.csv
    The status should be failure
    The output should equal ''
    The stderr should include 'failed to import'
  End

  It 'fails --inspect on a partial import (#300)'
    When run sqly --inspect testdata/user.csv /no/such/file.csv
    The status should be failure
    The output should equal ''
    The stderr should include 'failed to import'
  End

  It 'fails batch .import on a partial import and stops later commands (#302)'
    Data
      #|.import testdata/user.csv /no/such/file.csv
      #|.tables
    End
    When run sqly
    The status should be failure
    The output should not include 'TABLE NAME'
    The stderr should include 'failed to import'
  End

  It 'keeps stdout clean when a stdin dataset fails to import (#306)'
    Data
      #|
    End
    When run sqly --stdin csv --json --sql "SELECT COUNT(*) FROM stdin"
    The status should be failure
    The output should equal ''
    The stderr should include 'Import failed'
  End
End
