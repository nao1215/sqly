#!/bin/sh
# shellcheck shell=sh
#
# Quoting and escaping of space-containing paths for .import. These batch-mode
# tests pin the parsing contract at the binary level: a space-containing path
# must reach .import as one argument when escaped or quoted, and the unescaped
# form must fail.

Describe 'sqly .import with space-containing paths'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'imports a backslash-escaped space path as a single argument'
    Data
      #|.import testdata/space\ name.csv
      #|SELECT label FROM space_name ORDER BY score;
    End
    When run sqly
    The status should be success
    The output should include 'alpha'
    The output should include 'beta'
  End

  It 'imports a double-quoted space path as a single argument'
    Data
      #|.import "testdata/space name.csv"
      #|SELECT label FROM space_name ORDER BY score;
    End
    When run sqly
    The status should be success
    The output should include 'alpha'
  End

  It 'imports a single-quoted space path as a single argument'
    Data
      #|.import 'testdata/space name.csv'
      #|SELECT label FROM space_name ORDER BY score;
    End
    When run sqly
    The status should be success
    The output should include 'alpha'
  End

  It 'splits an unquoted space path into two failing arguments'
    Data
      #|.import testdata/space name.csv
    End
    When run sqly
    The status should be failure
    The stderr should include 'space'
  End
End
