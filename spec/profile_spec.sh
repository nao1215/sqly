#!/bin/sh
# shellcheck shell=sh
#
# CLI-first profile workflow (issue #275). --profile prints a data-quality report
# for every imported table: row/column counts, per-column null/blank/distinct/
# numeric counts, and warnings for mixed types, null-like text, and whitespace.
# JSON is the default automation contract; --profile-format text is human output.

Describe 'sqly --profile workflow'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORKDIR=$(mktemp -d)
    printf 'id,score,note\n1,10, hi \n2,abc,\n3,30,N/A\n' > "$WORKDIR/messy.csv"
    printf 'oid,amount\n1,9.99\n2,5.00\n' > "$WORKDIR/orders.csv"
  }
  cleanup() {
    rm -rf "$WORKDIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'reports per-column data quality as JSON'
    When run sqly --profile "$WORKDIR/messy.csv"
    The status should be success
    The output should include '"row_count": 3'
    The output should include '"column_count": 3'
    The output should include 'mixed numeric and non-numeric'
    The output should include 'null placeholders'
  End

  It 'profiles a stdin dataset'
    Data
      #|id,name
      #|1,Alice
      #|2,Bob
    End
    When run sqly --stdin csv --profile
    The status should be success
    The output should include '"name": "stdin"'
    The output should include '"row_count": 2'
  End

  It 'profiles multiple tables in one run'
    When run sqly --profile "$WORKDIR/messy.csv" "$WORKDIR/orders.csv"
    The status should be success
    The output should include '"name": "messy"'
    The output should include '"name": "orders"'
  End

  It 'emits a human-readable summary with --profile-format text'
    When run sqly --profile --profile-format text "$WORKDIR/orders.csv"
    The status should be success
    The output should include 'table orders: 2 rows, 2 columns'
  End

  It 'counts a blank string as a distinct value in JSON output'
    # Column v mixes one blank with one real value, so distinct_count is 2 and
    # stays consistent with blank_count instead of dropping the blank. Column id
    # has distinct_count 1, so the asserted value is unambiguous.
    printf 'id,v\nx,\nx,A\n' > "$WORKDIR/blank.csv"
    When run sqly --profile "$WORKDIR/blank.csv"
    The status should be success
    The output should include '"blank_count": 1'
    The output should include '"distinct_count": 2'
  End

  It 'counts a blank string as a distinct value in text output'
    printf 'id,v\nx,\nx,A\n' > "$WORKDIR/blank.csv"
    When run sqly --profile --profile-format text "$WORKDIR/blank.csv"
    The status should be success
    The output should include 'blanks=1 distinct=2'
  End

  It 'flags a padded null-like placeholder and its whitespace together'
    printf 'v\n" NULL "\n' > "$WORKDIR/nullspace.csv"
    When run sqly --profile "$WORKDIR/nullspace.csv"
    The status should be success
    The output should include 'null placeholders'
    The output should include 'leading or trailing whitespace'
  End

  It 'warns only about whitespace for a padded ordinary value'
    printf 'v\n" hello "\n' > "$WORKDIR/padded.csv"
    When run sqly --profile "$WORKDIR/padded.csv"
    The status should be success
    The output should not include 'null placeholders'
    The output should include 'leading or trailing whitespace'
  End
End
