#!/bin/sh
# shellcheck shell=sh
#
# CLI-first compare workflow (issue #276). --compare diffs two imported tables at
# the main command surface: schema, row count, and (with --compare-key) keyed
# rows. JSON is the default automation contract; --compare-format text is the
# human-readable option.

Describe 'sqly --compare workflow'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORKDIR=$(mktemp -d)
    printf 'id,name,age\n1,Alice,30\n2,Bob,25\n3,Carol,40\n' > "$WORKDIR/rev1.csv"
    printf 'id,name,age\n1,Alice,31\n2,Bob,25\n4,Dave,50\n' > "$WORKDIR/rev2.csv"
  }
  cleanup() {
    rm -rf "$WORKDIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'reports schema, row count, and keyed rows as JSON'
    When run sqly --compare --compare-key id "$WORKDIR/rev1.csv" "$WORKDIR/rev2.csv"
    The status should be success
    The output should include '"equal": true'
    The output should include '"delta": 0'
    The output should include '"key": "id"'
    # Carol removed, Dave added, Alice modified.
    The output should include '"4"'
    The output should include '"3"'
  End

  It 'emits a human-readable summary with --compare-format text'
    When run sqly --compare --compare-key id --compare-format text "$WORKDIR/rev1.csv" "$WORKDIR/rev2.csv"
    The status should be success
    The output should include 'schema: identical'
    The output should include '1 added, 1 removed, 1 modified'
  End

  It 'resolves an uppercase --compare-key against a lowercase column'
    # Header column is "id"; the uppercase spelling must resolve the same column
    # because SQLite identifier matching is case-insensitive.
    When run sqly --compare --compare-key ID "$WORKDIR/rev1.csv" "$WORKDIR/rev2.csv"
    The status should be success
    The output should include '"key": "ID"'
    The output should include '"4"'
    The output should include '"3"'
  End

  It 'rejects a missing key column'
    When run sqly --compare --compare-key nope "$WORKDIR/rev1.csv" "$WORKDIR/rev2.csv"
    The status should be failure
    The stderr should include 'compare key'
  End

  It 'resolves uppercase --compare-tables against lowercase table names'
    # Tables import as "user" and "identifier"; the uppercase pair must resolve
    # the same tables because SQLite identifier matching is case-insensitive.
    cp "$PROJECT_ROOT/testdata/user.csv" "$WORKDIR/user.csv"
    cp "$PROJECT_ROOT/testdata/identifier.csv" "$WORKDIR/identifier.csv"
    When run sqly --compare --compare-format text --compare-tables "USER,IDENTIFIER" "$WORKDIR/user.csv" "$WORKDIR/identifier.csv"
    The status should be success
    The line 1 should equal 'compare user -> identifier'
  End

  It 'rejects a genuinely missing --compare-tables name'
    When run sqly --compare --compare-tables "nope,rev2" "$WORKDIR/rev1.csv" "$WORKDIR/rev2.csv"
    The status should be failure
    The stderr should include 'compare table'
  End

  It 'rejects an ambiguous single-table compare'
    When run sqly --compare "$WORKDIR/rev1.csv"
    The status should be failure
    The stderr should include 'exactly two tables'
  End

  It 'follows CLI input order for left and right, not table-name sorting'
    # zebra is given first, ant second; the report must keep that direction even
    # though "ant" sorts before "zebra".
    printf 'id,name\n1,Alice\n' > "$WORKDIR/zebra.csv"
    printf 'id,name\n1,Alice\n' > "$WORKDIR/ant.csv"
    When run sqly --compare --compare-format text "$WORKDIR/zebra.csv" "$WORKDIR/ant.csv"
    The status should be success
    The line 1 should equal 'compare zebra -> ant'
  End

  It 'reverses left and right when the inputs are swapped'
    printf 'id,name\n1,Alice\n' > "$WORKDIR/zebra.csv"
    printf 'id,name\n1,Alice\n' > "$WORKDIR/ant.csv"
    When run sqly --compare --compare-format text "$WORKDIR/ant.csv" "$WORKDIR/zebra.csv"
    The status should be success
    The line 1 should equal 'compare ant -> zebra'
  End
End
