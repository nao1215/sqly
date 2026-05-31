#!/bin/sh
# shellcheck shell=sh
#
# Export format and compression inference from --output and .dump paths (#260).
# Verifies the binary infers the format from the destination extension, applies
# compression wrappers, round-trips compressed output, and reports conflicts.

Describe 'sqly export format inference (#260)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '--output path inference'
    It 'infers parquet from the destination extension without a flag'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --output "$out_dir/result.parquet"
      The status should be success
      The output should include 'output mode=parquet'
      The path "$out_dir/result.parquet" should be file
      rm -rf "$out_dir"
    End

    It 'infers ndjson with gzip and writes a compressed file'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --output "$out_dir/result.ndjson.gz"
      The status should be success
      The output should include 'output mode=ndjson'
      The path "$out_dir/result.ndjson.gz" should be file
      The contents of file "$out_dir/result.ndjson.gz" should not include 'booker12'
      rm -rf "$out_dir"
    End


    It 're-imports a gzip-compressed csv it wrote'
      out_dir=$(mktemp -d)
      export out_dir
      sqly --csv --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --output "$out_dir/result.csv.gz"
      When run sqly --csv --sql "SELECT user_name FROM result LIMIT 1" "$out_dir/result.csv.gz"
      The status should be success
      The line 1 should equal 'user_name'
      The line 2 should equal 'booker12'
      rm -rf "$out_dir"
    End
  End

  Describe 'unknown extension honors the exact path (#292)'
    It 'writes the CSV fallback to the requested path without rewriting it'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --sql "SELECT user_name FROM user LIMIT 1" testdata/user.csv --output "$out_dir/out.unknown"
      The status should be success
      The output should include "$out_dir/out.unknown"
      The path "$out_dir/out.unknown" should be file
      The path "$out_dir/out.csv" should not be exist
      rm -rf "$out_dir"
    End
  End

  Describe 'conflicts and unsupported combinations'
    It 'errors when an explicit mode flag disagrees with the path extension'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --json --sql "SELECT user_name FROM user LIMIT 1" testdata/user.csv --output "$out_dir/result.csv"
      The status should be failure
      The stderr should include 'conflicts with destination path'
      rm -rf "$out_dir"
    End

    It 'rejects bzip2 output'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --sql "SELECT user_name FROM user LIMIT 1" testdata/user.csv --output "$out_dir/result.csv.bz2"
      The status should be failure
      The stderr should include 'bzip2'
      rm -rf "$out_dir"
    End

    It 'rejects compression on parquet'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --sql "SELECT user_name FROM user LIMIT 1" testdata/user.csv --output "$out_dir/result.parquet.gz"
      The status should be failure
      The stderr should include 'cannot be compressed'
      rm -rf "$out_dir"
    End
  End

  Describe '.dump path inference'
    It 'infers tsv from the .dump destination path'
      out_dir=$(mktemp -d)
      export out_dir
      Data:expand
        #|.dump user ${out_dir}/dump.tsv
      End
      When run sqly testdata/user.csv
      The status should be success
      The output should include 'mode=tsv'
      The path "$out_dir/dump.tsv" should be file
      rm -rf "$out_dir"
    End
  End

  Describe 'backward compatibility'
    It 'keeps --json --output result.json writing json'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --json --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --output "$out_dir/result.json"
      The status should be success
      The output should include 'output mode=json'
      The path "$out_dir/result.json" should be file
      The contents of file "$out_dir/result.json" should include '"user_name":"booker12"'
      rm -rf "$out_dir"
    End
  End
End
