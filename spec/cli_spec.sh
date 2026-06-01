#!/bin/sh
# shellcheck shell=sh
#
# Broad CLI surface end-to-end tests: version/help, error handling and exit
# codes, multi-file JOINs, and writing results to a file with --output.

Describe 'sqly CLI surface'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  Describe '--version and --help'
    It 'prints the version'
      When run sqly --version
      The status should be success
      The output should include 'sqly'
    End

    It 'prints usage with --help'
      When run sqly --help
      The status should be success
      The output should include '[Usage]'
      The output should include '--json'
    End
  End

  Describe 'error handling'
    It 'fails on a non-existent file'
      When run sqly --sql "SELECT 1" testdata/does_not_exist.csv
      The status should be failure
      The stderr should include 'does not exist'
    End

    It 'fails on invalid SQL with --sql'
      When run sqly --sql "SELEKT * FROM user" testdata/user.csv
      The status should be failure
      The stderr should be present
    End
  End

  Describe 'multi-file JOIN'
    It 'joins two imported files'
      When run sqly --csv --sql "SELECT user_name, position FROM user INNER JOIN identifier ON user.identifier = identifier.id ORDER BY user.identifier LIMIT 1" testdata/user.csv testdata/identifier.csv
      The status should be success
      The line 1 should equal 'user_name,position'
      The output should include 'booker12'
    End
  End

  Describe '--output to file'
    It 'writes JSON results to the given path'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --json --output "$out_dir/result.json" --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv
      The status should be success
      The stderr should include 'result.json'
      The path "$out_dir/result.json" should be file
      The contents of file "$out_dir/result.json" should include '"user_name":"booker12"'
      rm -rf "$out_dir"
    End
  End

  Describe 'flags after input paths'
    It 'applies --output placed after the file path instead of importing it'
      out_dir=$(mktemp -d)
      export out_dir
      When run sqly --json --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --output "$out_dir/result.json"
      The status should be success
      The stderr should include 'result.json'
      The output should not include 'path does not exist'
      The path "$out_dir/result.json" should be file
      The contents of file "$out_dir/result.json" should include '"user_name":"booker12"'
      rm -rf "$out_dir"
    End

    It 'applies an output-mode flag placed after the file path'
      When run sqly --sql "SELECT user_name FROM user ORDER BY identifier LIMIT 1" testdata/user.csv --csv
      The status should be success
      The line 1 should equal 'user_name'
      The output should include 'booker12'
      The output should not include 'path does not exist'
    End

    It 'fails fast on an unknown flag after the file path'
      When run sqly testdata/user.csv --nope
      The status should be failure
      The stderr should include 'unknown flag'
    End
  End
End
