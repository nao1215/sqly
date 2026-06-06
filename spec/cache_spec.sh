#!/bin/sh
# shellcheck shell=sh
#
# Opt-in import cache (issue #284). --cache snapshots the imported tables to a
# SQLite file; a warm run with unchanged inputs reloads from it instead of
# re-parsing the sources. The cache invalidates when the source changes, can be
# cleared with --cache-clear, and a cache failure falls back to a cold import.

Describe 'sqly --cache import cache'
  Include "$SHELLSPEC_SPECDIR/spec_helper.sh"

  setup() {
    WORKDIR=$(mktemp -d)
    printf 'id,name\n1,Alice\n2,Bob\n3,Carol\n' > "$WORKDIR/data.csv"
    CACHE="$WORKDIR/snap.cache"
  }
  cleanup() {
    rm -rf "$WORKDIR"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'writes a cache on the cold run and reuses it on the warm run'
    When run sh -c "'$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv' >/dev/null && '$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv'"
    The status should be success
    The output should include '3'
    The stderr should include 'cache: reused'
  End

  It 'invalidates the cache when the source changes'
    When run sh -c "'$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv' >/dev/null && printf 'id,name\n1,Alice\n2,Bob\n3,Carol\n4,Dave\n' > '$WORKDIR/data.csv' && '$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv'"
    The status should be success
    The output should include '4'
    # A rebuild does not print the reuse banner.
    The stderr should not include 'cache: reused'
  End

  It 'falls back to a cold import when the cache path is unwritable'
    # A non-empty directory at the cache path cannot be written as a SQLite file.
    When run sh -c "mkdir -p '$CACHE' && touch '$CACHE/keep' && '$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv'"
    The status should be success
    The output should include '3'
    The stderr should include 'cache'
  End

  It 'rebuilds after --cache-clear'
    When run sh -c "'$SQLY_BIN' --cache '$CACHE' --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv' >/dev/null && '$SQLY_BIN' --cache '$CACHE' --cache-clear --sql 'SELECT COUNT(*) AS n FROM data' '$WORKDIR/data.csv'"
    The status should be success
    The output should include '3'
    The stderr should not include 'cache: reused'
  End
End
