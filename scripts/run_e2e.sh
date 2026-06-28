#!/bin/sh
# shellcheck shell=sh
#
# Hermetic ShellSpec runner for sqly. It builds the binary and runs the E2E
# suite inside a throwaway temp-backed HOME and config sandbox, so the suite
# never reads or writes the developer's real config directory and local and CI
# runs are identical. Any extra arguments are forwarded to shellspec (for
# example a single spec file).
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Build the binary the specs exercise. spec_helper.sh resolves it at $ROOT/sqly.
make build

# Create an isolated sandbox and remove it on exit, so no run leaves state behind.
SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT INT TERM

mkdir -p "$SANDBOX/home" "$SANDBOX/config" "$SANDBOX/data" "$SANDBOX/cache"

# Point HOME and every XDG base directory at the sandbox so config, history, and
# cache files land there instead of in the developer's real home. USERPROFILE
# covers Windows-style home resolution if the suite ever runs there.
HOME="$SANDBOX/home"
export HOME
export USERPROFILE="$SANDBOX/home"
export XDG_CONFIG_HOME="$SANDBOX/config"
export XDG_DATA_HOME="$SANDBOX/data"
export XDG_CACHE_HOME="$SANDBOX/cache"

# Route sqly's command history to the sandbox explicitly, so specs that do not
# set their own SQLY_HISTORY_DB_PATH still never touch the real history DB.
export SQLY_HISTORY_DB_PATH="$SANDBOX/history.db"

# Expose the sandbox root so the hermeticity spec can assert that HOME and the
# history DB live inside it.
export SQLY_E2E_SANDBOX="$SANDBOX"

exec shellspec --shell sh "$@"
