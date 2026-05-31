#!/bin/sh
# shellcheck shell=sh
#
# Non-interactive inspect workflow end-to-end tests (#259). Runs the binary with
# --inspect and checks the machine-readable JSON report and stdout purity.

Describe 'sqly --inspect (#259)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'prints a JSON report for a single file'
    When run sqly --inspect testdata/user.csv
    The status should be success
    The line 1 should equal '{'
    The output should include '"name": "user"'
    The output should include 'testdata/user.csv'
    The output should include '"row_count": 3'
    The output should include '"user_name"'
  End

  It 'maps every table from a multi-table ACH file to its source'
    When run sqly --inspect testdata/ppd-debit.ach
    The status should be success
    The output should include '"name": "ppd_debit_entries"'
    The output should include 'ppd-debit.ach'
  End

  It 'keeps stdout as pure JSON for a directory and sends progress to stderr'
    work_dir=$(mktemp -d)
    export work_dir
    cp testdata/user.csv "$work_dir/a.csv"
    cp testdata/identifier.csv "$work_dir/b.csv"
    When run sqly --inspect "$work_dir"
    The status should be success
    The line 1 should equal '{'
    The output should include '"name": "a"'
    The output should include '"name": "b"'
    # Each table reports its real source file, not the directory path (#326).
    The output should include '/a.csv'
    The output should include '/b.csv'
    The stderr should include 'Successfully imported'
    rm -rf "$work_dir"
  End

  It 'fails with a clear error when no input is given'
    When run sqly --inspect
    The status should be failure
    The stderr should include 'no tables to inspect'
  End

  It 'emits a schema-only report with --inspect-sample 0'
    When run sqly --inspect --inspect-sample 0 testdata/user.csv
    The status should be success
    The line 1 should equal '{'
    The output should include '"name": "user"'
    The output should include '"user_name"'
    The output should include '"sample_rows": []'
    The output should not include 'booker12'
  End

  It 'limits sample rows with --inspect-sample'
    When run sqly --inspect --inspect-sample 1 testdata/user.csv
    The status should be success
    The output should include 'booker12'
    The output should not include 'jenkins46'
  End

  It 'rejects a negative --inspect-sample'
    When run sqly --inspect --inspect-sample -1 testdata/user.csv
    The status should be failure
    The stderr should include 'inspect-sample'
  End
End
