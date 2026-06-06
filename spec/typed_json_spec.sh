#!/bin/sh
# shellcheck shell=sh
#
# Typed JSON output contract (issue #282). --json-typed/--ndjson-typed emit
# native JSON scalars (numbers, booleans, nulls) instead of strings, while the
# default --json/--ndjson keep the legacy string contract. A large integer from
# an imported column stays lossless and never regresses into scientific notation.

Describe 'sqly typed JSON output (--json-typed / --ndjson-typed)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'emits native numbers, booleans, and null with --json-typed'
    When run sqly --json-typed --sql "SELECT 42 AS i, -1.5 AS f, NULL AS n, 'x' AS s"
    The status should be success
    The output should include '"i":42'
    The output should include '"f":-1.5'
    The output should include '"n":null'
    The output should include '"s":"x"'
  End

  It 'keeps the legacy string contract with plain --json'
    When run sqly --json --sql "SELECT 42 AS i"
    The status should be success
    The output should include '"i":"42"'
  End

  It 'emits native scalars per line with --ndjson-typed'
    When run sqly --ndjson-typed --sql "SELECT 7 AS n, 't' AS s"
    The status should be success
    The output should include '"n":7'
    The output should include '"s":"t"'
  End

  It 'keeps a large integer column lossless (no scientific notation)'
    When run sqly --json-typed --sql "SELECT amount FROM typed_bigint WHERE id = 1" testdata/typed_bigint.csv
    The status should be success
    The output should include '"amount":9007199254740993'
    The output should not include 'e+'
  End

  It 'leaves a leading-zero value as a string'
    When run sqly --json-typed --sql "SELECT '007' AS code"
    The status should be success
    The output should include '"code":"007"'
  End

  It 'uses the typed contract for --inspect sample rows'
    When run sqly --inspect --json-typed testdata/typed_bigint.csv
    The status should be success
    The output should include '"amount": 9007199254740993'
  End

  It 'rejects plain --json combined with --inspect'
    When run sqly --inspect --json testdata/typed_bigint.csv
    The status should be failure
    The stderr should include 'inspect'
  End
End
