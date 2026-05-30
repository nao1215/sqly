#!/bin/sh
# shellcheck shell=sh
#
# shellspec helper for sqly end-to-end tests. These exercise the built binary
# the way a user does (flags, piped stdin, exit codes) rather than internal Go
# paths, so they catch regressions unit tests cannot.

set -eu

PROJECT_ROOT="$(cd "$SHELLSPEC_SPECDIR/.." && pwd)"
export PROJECT_ROOT

# SQLY_BIN points at the binary built by `make build`. Override to test another
# build. The specs fail loudly (below) if it is missing.
SQLY_BIN="${SQLY_BIN:-$PROJECT_ROOT/sqly}"
export SQLY_BIN

# sqly runs the built binary from the project root so that testdata/ relative
# paths resolve regardless of where shellspec was invoked.
sqly() {
  cd "$PROJECT_ROOT" && "$SQLY_BIN" "$@"
}
