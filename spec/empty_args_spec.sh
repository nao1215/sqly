#!/bin/sh
# shellcheck shell=sh
#
# Empty quoted command arguments. An empty argument must be
# rejected, not reinterpreted as an in-place save, a ".csv" file, or the current
# directory.

Describe 'sqly empty command arguments'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORK=$(mktemp -d)
    cp testdata/user.csv "$WORK/u.csv"
  }
  cleanup() {
    rm -rf "$WORK"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'rejects .save "" and leaves the source unchanged'
    Data
      #|UPDATE u SET first_name = 'EMPTY' WHERE identifier = 1;
      #|.save ""
    End
    When run sqly "$WORK/u.csv"
    The status should be failure
    The output should include 'affected'
    The stderr should include '.save requires'
    The contents of file "$WORK/u.csv" should not include 'EMPTY'
  End

  It 'rejects .dump with an empty destination'
    Data
      #|.dump user ""
    End
    When run sqly testdata/user.csv
    The status should be failure
    The stderr should include '.dump requires'
    The path .csv should not be exist
  End

  It 'rejects .import with an empty path'
    Data
      #|.import ""
      #|.tables
    End
    When run sqly
    The status should be failure
    The output should not include 'TABLE NAME'
    The stderr should include 'empty import path'
  End
End
