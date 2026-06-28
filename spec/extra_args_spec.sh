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

  It 'rejects .pwd with an extra argument'
    Data
      #|.pwd extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.pwd'
  End

  It 'rejects .clear with an extra argument'
    Data
      #|.clear extra
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.clear'
  End

  It 'does not let .exit with an extra argument silently terminate the batch'
    Data
      #|.exit extra
      #|SELECT 1;
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.exit'
  End
End
