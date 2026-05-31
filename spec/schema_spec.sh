#!/bin/sh
# shellcheck shell=sh
#
# Schema inspection end-to-end tests (#238): .schema and .describe across CSV,
# JSON, Excel, and ACH-generated tables, plus JSON-mode structured output and
# missing-table errors. Driven through piped stdin against the built binary.

Describe 'sqly schema inspection'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '.schema'
    It 'prints a CREATE TABLE statement for a CSV table'
      Data
        #|.schema user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'CREATE TABLE'
      The output should include 'user_name'
    End

    It 'prints a CREATE TABLE statement for an ACH-generated table'
      Data
        #|.schema ppd_debit_entries
      End
      When run sqly testdata/ppd-debit.ach
      The status should be success
      The output should include 'CREATE TABLE'
      The output should include 'trace_number'
    End

    It 'emits a structured object in json mode'
      Data
        #|.mode json
        #|.schema user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include '"table":"user"'
      The output should include '"schema":"CREATE TABLE'
      The stderr should include 'Change output mode'
    End

    It 'errors on a missing table'
      Data
        #|.schema no_such_table
      End
      When run sqly testdata/user.csv
      The status should be failure
      The stderr should include 'no such table'
    End
  End

  Describe '.describe'
    It 'lists columns and types for a CSV table'
      Data
        #|.describe user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'user_name'
      The output should include 'identifier'
    End

    It 'describes an Excel-generated table'
      Data
        #|.describe sample_test_sheet
      End
      When run sqly testdata/sample.xlsx
      The status should be success
      The output should include 'name'
    End

    It 'describes a JSON-imported table as a single data column'
      Data
        #|.describe sample
      End
      When run sqly testdata/sample.json
      The status should be success
      The output should include 'data'
    End

    It 'emits structured column metadata in json mode'
      Data
        #|.mode json
        #|.describe user
      End
      When run sqly testdata/user.csv
      The status should be success
      # Pair name and type for the same column so the assertion verifies
      # user_name's own type rather than matching another column's INTEGER.
      The output should include '"name":"user_name","type":"TEXT"'
      The stderr should include 'Change output mode'
    End

    It 'errors on a missing table'
      Data
        #|.describe no_such_table
      End
      When run sqly testdata/user.csv
      The status should be failure
      The stderr should include 'no such table'
    End
  End
End
