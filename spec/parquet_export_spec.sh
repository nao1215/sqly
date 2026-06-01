#!/bin/sh
# shellcheck shell=sh
#
# Parquet export end-to-end tests. Parquet is export-only, like Excel:
# it is written by .dump and --output, re-imports cleanly, normalizes the file
# extension, and reports a clear error for an empty result.

Describe 'sqly parquet export'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '.dump and round-trip'
    check_dump_roundtrip() {
      dir=$(mktemp -d)
      out="$dir/user.parquet"
      printf '.mode parquet\n.dump user %s\n' "$out" | sqly testdata/user.csv >/dev/null
      [ -f "$out" ] || { rm -rf "$dir"; return 1; }
      # A fresh import of the parquet file must have the same row count.
      count=$(printf 'SELECT COUNT(*) FROM user\n' | sqly "$out" | tail -n +2 | grep -o '[0-9]\+' | head -1)
      rm -rf "$dir"
      [ "$count" = "3" ]
    }
    It 'writes a parquet file that re-imports with the same rows'
      When call check_dump_roundtrip
      The status should be success
      The stderr should include 'Change output mode'
    End
  End

  Describe 'extension normalization'
    check_extension() {
      dir=$(mktemp -d)
      # No extension given; parquet mode must produce a .parquet file.
      printf '.mode parquet\n.dump user %s/result\n' "$dir" | sqly testdata/user.csv >/dev/null
      ok=1
      [ -f "$dir/result.parquet" ] || ok=0
      rm -rf "$dir"
      [ "$ok" = "1" ]
    }
    It 'appends the .parquet extension'
      When call check_extension
      The status should be success
      The stderr should include 'Change output mode'
    End
  End

  Describe '--output to a parquet file'
    It 'writes query results to the given parquet path'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --parquet --output "$out_dir/q.parquet" --sql "SELECT user_name FROM user LIMIT 2" testdata/user.csv
      The status should be success
      The stderr should include 'q.parquet'
      The path "$out_dir/q.parquet" should be file
      rm -rf "$out_dir"
    End
  End

  Describe 'empty result'
    It 'reports a clear error when exporting an empty result'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --parquet --output "$out_dir/empty.parquet" --sql "SELECT user_name FROM user WHERE 1=0" testdata/user.csv
      The status should be failure
      The stderr should include 'empty result'
      rm -rf "$out_dir"
    End
  End
End
