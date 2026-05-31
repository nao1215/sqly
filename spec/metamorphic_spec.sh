#!/bin/sh
# shellcheck shell=sh
#
# Metamorphic end-to-end tests: relations that must hold between related runs of
# the binary (count vs rows, order invariance, format invariance, dump/reimport
# round-trip). These catch regressions that single fixed-output assertions miss.

Describe 'sqly metamorphic relations'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  # data_rows prints the number of data rows (CSV body) for a query.
  data_rows() {
    sqly --csv --sql "$1" testdata/user.csv | tail -n +2 | grep -c .
  }

  Describe 'COUNT(*) vs row count'
    check_count() {
      rows=$(data_rows "SELECT user_name FROM user")
      count=$(sqly --csv --sql "SELECT COUNT(*) FROM user" testdata/user.csv | tail -n +2)
      [ "$rows" = "$count" ]
    }
    It 'COUNT(*) equals the number of selected rows'
      When call check_count
      The status should be success
    End
  End

  Describe 'WHERE tautology / contradiction'
    check_where() {
      all=$(data_rows "SELECT user_name FROM user")
      tautology=$(data_rows "SELECT user_name FROM user WHERE 1=1")
      contradiction=$(data_rows "SELECT user_name FROM user WHERE 1=0")
      [ "$all" = "$tautology" ] && [ "$contradiction" = "0" ]
    }
    It 'WHERE 1=1 returns all rows and WHERE 1=0 returns none'
      When call check_where
      The status should be success
    End
  End

  Describe 'ORDER BY is a permutation'
    check_order() {
      unordered=$(sqly --csv --sql "SELECT user_name FROM user" testdata/user.csv | tail -n +2 | sort)
      ordered=$(sqly --csv --sql "SELECT user_name FROM user ORDER BY user_name DESC" testdata/user.csv | tail -n +2 | sort)
      [ "$unordered" = "$ordered" ]
    }
    It 'ORDER BY preserves the row multiset'
      When call check_order
      The status should be success
    End
  End

  Describe 'output format invariance'
    check_format() {
      csv_rows=$(sqly --csv --sql "SELECT user_name FROM user" testdata/user.csv | tail -n +2 | grep -c .)
      ndjson_rows=$(sqly --ndjson --sql "SELECT user_name FROM user" testdata/user.csv | grep -c .)
      [ "$csv_rows" = "$ndjson_rows" ]
    }
    It 'csv and ndjson yield the same number of rows'
      When call check_format
      The status should be success
    End
  End

  Describe 'dump then reimport round-trip'
    check_roundtrip() {
      dir=$(mktemp -d)
      out="$dir/rt.csv"
      printf '.dump user %s\n' "$out" | sqly testdata/user.csv >/dev/null 2>&1
      original=$(sqly --csv --sql "SELECT * FROM user ORDER BY identifier" testdata/user.csv)
      reimported=$(sqly --csv --sql "SELECT * FROM rt ORDER BY identifier" "$out")
      rm -rf "$dir"
      [ "$original" = "$reimported" ]
    }
    It 'CSV dump reimported yields identical data'
      When call check_roundtrip
      The status should be success
    End
  End
End
