#!/bin/sh
# shellcheck shell=sh
#
# Runs the known-bug specs in e2e/atago/known_bugs/. These specs describe the
# behavior sqly should have once the underlying filesql dialect bugs are fixed,
# so they are expected to FAIL today. They are deliberately kept out of
# scripts/run_e2e.sh (and therefore out of CI) so the default suite stays green
# while the fixes land.
#
# Usage:
#   sh scripts/run_known_bugs.sh                 # run every known-bug spec
#   sh scripts/run_known_bugs.sh --filter mysql  # focus on one group
#
# A scenario that starts passing is a fix landing: move it into the matching
# e2e/atago/*.atago.yaml suite so CI protects it from regressing again.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v atago >/dev/null 2>&1; then
	echo "known-bugs: atago is not installed. Install it from https://github.com/nao1215/atago" >&2
	exit 127
fi

make build

# Same throwaway sandbox as scripts/run_e2e.sh, so a known-bug run never reads or
# writes the developer's real config, history, or cache directories.
SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT INT TERM

mkdir -p "$SANDBOX/home" "$SANDBOX/config" "$SANDBOX/data" "$SANDBOX/cache" "$SANDBOX/bin"
cp "$ROOT/sqly" "$SANDBOX/bin/sqly"

HOME="$SANDBOX/home"
export HOME
export USERPROFILE="$SANDBOX/home"
export XDG_CONFIG_HOME="$SANDBOX/config"
export XDG_DATA_HOME="$SANDBOX/data"
export XDG_CACHE_HOME="$SANDBOX/cache"
export SQLY_HISTORY_PATH="$SANDBOX/history"

PATH="$SANDBOX/bin:$PATH"
export PATH

# No --ci: a run that reports failures is the expected outcome here, and the
# script should print them all rather than stop at the first spec.
atago run "$@" "$ROOT"/e2e/atago/known_bugs/*.atago.yaml
