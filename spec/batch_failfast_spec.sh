#!/bin/sh
# shellcheck shell=sh
#
# Batch fail-fast semantics (#308, #320, #321, #322, #330, #331). The first
# failed statement stops the batch, so later statements and side-effecting
# helper commands (.save, .dump) do not run, and an empty batch performs no
# write-back.

Describe 'sqly batch fail-fast (#308)'
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

  It 'stops later statements after a SQL failure'
    Data
      #|SELECT * FROM no_such_table;
      #|SELECT 1 AS later;
    End
    When run sqly testdata/user.csv
    The status should be failure
    The output should not include 'later'
    The stderr should include 'no_such_table'
  End

  It 'does not run .save --force after an earlier failure (#320)'
    Data:expand
      #|UPDATE u SET first_name = 'BROKEN' WHERE identifier = 1;
      #|SELECT * FROM no_such_table;
      #|.save --force
    End
    When run sqly "$WORK/u.csv"
    The status should be failure
    The output should include 'affected'
    The stderr should not include 'Saved'
    The contents of file "$WORK/u.csv" should not include 'BROKEN'
  End

  It 'does not run .dump after an earlier failure (#322)'
    Data:expand
      #|SELECT * FROM no_such_table;
      #|.dump u ${WORK}/out.csv
    End
    When run sqly "$WORK/u.csv"
    The status should be failure
    The stderr should include 'no_such_table'
    The path "$WORK/out.csv" should not be exist
  End

  It 'does not write back for empty stdin with --save --force (#330)'
    When run sqly "$WORK/u.csv" --save --force
    The status should be success
    The stderr should not include 'Saved'
    The contents of file "$WORK/u.csv" should equal "$(cat testdata/user.csv)"
  End
End
