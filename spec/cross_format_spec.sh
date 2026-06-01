#!/bin/sh
# shellcheck shell=sh
#
# End-to-end coverage for the README demos that mix file formats: a JOIN across
# a Parquet and a CSV table, --output used as a format converter, and a
# directory import that joins files of different formats in one query.

Describe 'sqly cross-format workflows'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe 'cross-format JOIN (Parquet x CSV)'
    It 'joins a Parquet table to a CSV table and computes a column'
      When call sqly --sql "SELECT p.name, p.price, s.quantity, ROUND(p.price * s.quantity, 2) AS revenue FROM products p JOIN sales s ON p.id = s.product_id ORDER BY revenue DESC" testdata/products.parquet testdata/sales.csv
      The status should be success
      The output should include 'revenue'
      The output should include 'Laptop'
      The output should include '2999.97'
    End
  End

  Describe '--output as a format converter'
    # Query one CSV and write JSON, Parquet, and Excel; the Parquet result must
    # re-import as a normal table with the same rows.
    convert_roundtrip() {
      dir=$(mktemp -d)
      sqly --sql "SELECT user_name, identifier FROM user" --output "$dir/users.json" testdata/user.csv >/dev/null 2>&1
      sqly --sql "SELECT user_name, identifier FROM user" --output "$dir/users.parquet" testdata/user.csv >/dev/null 2>&1
      sqly --sql "SELECT user_name, identifier FROM user" --output "$dir/users.xlsx" testdata/user.csv >/dev/null 2>&1
      ok=1
      [ -s "$dir/users.json" ] || ok=0
      [ -s "$dir/users.parquet" ] || ok=0
      [ -s "$dir/users.xlsx" ] || ok=0
      count=$(printf 'SELECT COUNT(*) FROM users\n' | sqly "$dir/users.parquet" 2>/dev/null | grep -o '[0-9]\+' | head -1)
      [ "$count" = "3" ] || ok=0
      rm -rf "$dir"
      [ "$ok" = "1" ]
    }
    It 'writes json, parquet, and excel and re-queries the parquet'
      When call convert_roundtrip
      The status should be success
    End
  End

  Describe 'directory import across formats'
    # A folder holding a Parquet file and a CSV file must import as two tables
    # that join in a single query.
    join_directory() {
      dir=$(mktemp -d)
      cp "$PROJECT_ROOT/testdata/products.parquet" "$PROJECT_ROOT/testdata/sales.csv" "$dir/"
      sqly "$dir" --sql "SELECT p.name, s.quantity FROM products p JOIN sales s ON p.id = s.product_id ORDER BY p.name"
      status=$?
      rm -rf "$dir"
      return $status
    }
    It 'joins a Parquet and a CSV file from one directory argument'
      When call join_directory
      The status should be success
      The output should include 'Keyboard'
      The output should include 'quantity'
      The stderr should include 'imported'
    End
  End
End
