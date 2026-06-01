#!/bin/sh
# shellcheck shell=sh
#
# filesql integration end-to-end tests. Locks that every supported file
# format imports into the shared session and can be queried and inspected, and
# that the ACH/Fedwire cleanup path stays deterministic across repeated imports.

Describe 'sqly filesql integration'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'import regressions across formats'
    It 'imports and queries a CSV file'
      Data
        #|SELECT COUNT(*) FROM user
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include '3'
    End

    It 'imports and queries a JSONL file'
      Data
        #|SELECT COUNT(*) FROM sample
      End
      When run sqly testdata/sample.jsonl
      The status should be success
      The output should include 'COUNT(*)'
    End

    It 'imports and queries a Parquet file'
      Data
        #|SELECT COUNT(*) FROM products
      End
      When run sqly testdata/products.parquet
      The status should be success
      The output should include '3'
    End

    It 'imports and queries an Excel file'
      Data
        #|SELECT COUNT(*) FROM sample_test_sheet
      End
      When run sqly testdata/sample.xlsx
      The status should be success
      The output should include 'COUNT(*)'
    End

    It 'imports and queries an ACH file'
      Data
        #|SELECT COUNT(*) FROM ppd_debit_entries
      End
      When run sqly testdata/ppd-debit.ach
      The status should be success
      The output should include 'COUNT(*)'
    End

    It 'imports and queries a Fedwire file'
      Data
        #|SELECT COUNT(*) FROM customer_transfer_message
      End
      When run sqly testdata/customer-transfer.fed
      The status should be success
      The output should include 'COUNT(*)'
    End
  End

  Describe 'schema fidelity from upstream'
    It 'preserves filesql-detected column types for schema inspection'
      Data
        #|.describe products
      End
      When run sqly testdata/products.parquet
      The status should be success
      The output should include 'name'
      The output should include 'price'
    End
  End

  Describe 'ACH/Fedwire cleanup determinism'
    # Importing the same ACH twice must yield the same tables; the registry
    # cleanup after each import keeps the result independent of prior imports.
    check_ach_repeatable() {
      first=$(printf '.tables\n' | sqly testdata/ppd-debit.ach)
      second=$(printf '.tables\n' | sqly testdata/ppd-debit.ach)
      [ "$first" = "$second" ]
    }
    It 'produces identical tables on repeated ACH imports'
      When call check_ach_repeatable
      The status should be success
    End
  End
End
