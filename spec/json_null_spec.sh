#!/bin/sh
# shellcheck shell=sh
#
# JSON/NDJSON NULL rendering (#328, #329). A SQL NULL must be emitted as JSON
# null, distinct from an empty string, in machine-readable output.

Describe 'sqly JSON/NDJSON NULL handling (#328, #329)'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  It 'emits NULL as JSON null in --json'
    When run sqly --json --sql "SELECT NULL AS n, '' AS e, 1 AS x"
    The status should be success
    The output should include '"n":null'
    The output should include '"e":""'
  End

  It 'emits NULL as JSON null in --ndjson'
    When run sqly --ndjson --sql "SELECT NULL AS n, '' AS e, 1 AS x"
    The status should be success
    The output should include '"n":null'
    The output should include '"e":""'
  End
End
