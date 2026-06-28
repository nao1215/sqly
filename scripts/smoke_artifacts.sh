#!/bin/sh
# shellcheck shell=sh
#
# Validates GoReleaser snapshot artifacts before a real release tag is cut. It
# assumes `goreleaser release --snapshot` has already populated dist/ (the CI job
# runs GoReleaser, then this script). It checks that the expected archives and OS
# packages exist, that the host archive extracts, and that the extracted binary
# runs. A successful `go build` does not prove published artifacts are usable;
# this catches archive-layout and packaging regressions at PR time.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

if [ ! -d "$DIST" ]; then
  echo "smoke: dist/ not found; run 'goreleaser release --snapshot --clean' first" >&2
  exit 1
fi

fail=0

# require_glob NAME PATTERN: fail if no file matches PATTERN under dist/.
require_glob() {
  name="$1"
  pattern="$2"
  # shellcheck disable=SC2086 # intentional glob expansion
  set -- $pattern
  if [ -e "$1" ]; then
    echo "ok: $name present ($1)"
  else
    echo "MISSING: $name ($pattern)" >&2
    fail=1
  fi
}

require_glob "linux tar.gz archive" "$DIST/*linux*.tar.gz"
require_glob "darwin tar.gz archive" "$DIST/*darwin*.tar.gz"
require_glob "windows zip archive" "$DIST/*windows*.zip"
require_glob "checksums" "$DIST/checksums.txt"
require_glob "deb package" "$DIST/*.deb"
require_glob "rpm package" "$DIST/*.rpm"
require_glob "apk package" "$DIST/*.apk"

# Extract the host (linux amd64) archive and run the binary to prove it executes
# after packaging, not just that `go build` succeeded.
host_archive=""
for f in "$DIST"/*linux_amd64*.tar.gz; do
  [ -e "$f" ] || continue
  host_archive="$f"
  break
done
if [ -n "$host_archive" ]; then
  work=$(mktemp -d)
  tar -xzf "$host_archive" -C "$work"
  if "$work/sqly" --version | grep -q "sqly"; then
    echo "ok: extracted binary runs ($("$work/sqly" --version))"
  else
    echo "MISSING: extracted binary did not report a version" >&2
    fail=1
  fi
  rm -rf "$work"
else
  echo "MISSING: linux_amd64 archive to extract" >&2
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  echo "artifact smoke checks FAILED" >&2
  exit 1
fi
echo "artifact smoke checks passed"
