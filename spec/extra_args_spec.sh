#!/bin/sh
# shellcheck shell=sh
#
# Helper commands reject unexpected extra arguments. Trailing arguments
# must cause a clear error, not be silently ignored.

Describe 'sqly helper commands reject extra args'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'rejects .schema with an extra argument'
    Data
      #|.schema user extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.schema'
  End

  It 'rejects .describe with an extra argument'
    Data
      #|.describe user extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.describe'
  End

  It 'rejects .tables with an extra argument'
    Data
      #|.tables extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.tables'
  End

  It 'rejects .mode with an extra argument'
    Data
      #|.mode csv extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.mode'
  End
End
