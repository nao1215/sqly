#!/bin/sh
# shellcheck shell=sh
#
# Hermetic atago runner for sqly. It builds the binary and runs the E2E suite
# (e2e/atago/*.atago.yaml) inside a throwaway temp-backed HOME and config
# sandbox, so the suite never reads or writes the developer's real config
# directory and local and CI runs are identical. The tests themselves are
# plain-YAML atago specs; this script is only the environment bootstrap. Any
# extra arguments are forwarded to `atago run` (for example `--filter cache`).
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v atago >/dev/null 2>&1; then
	echo "e2e: atago is not installed. Install it from https://github.com/nao1215/atago" >&2
	echo "e2e: e.g. 'go install github.com/nao1215/atago@latest' (CI uses nao1215/setup-atago)" >&2
	exit 127
fi

# Build the binary the specs exercise; it is exposed on PATH below.
make build

# Create an isolated sandbox and remove it on exit, so no run leaves state behind.
SANDBOX="$(mktemp -d)"
trap 'rm -rf "$SANDBOX"' EXIT INT TERM

mkdir -p "$SANDBOX/home" "$SANDBOX/config" "$SANDBOX/data" "$SANDBOX/cache" "$SANDBOX/bin"
cp "$ROOT/sqly" "$SANDBOX/bin/sqly"

# Point HOME and every XDG base directory at the sandbox so config, history, and
# cache files land there instead of in the developer's real home. USERPROFILE
# covers Windows-style home resolution if the suite ever runs there. Scenarios
# that need finer isolation set their own HOME/paths via `env:` + ${workdir}.
HOME="$SANDBOX/home"
export HOME
export USERPROFILE="$SANDBOX/home"
export XDG_CONFIG_HOME="$SANDBOX/config"
export XDG_DATA_HOME="$SANDBOX/data"
export XDG_CACHE_HOME="$SANDBOX/cache"

# Route sqly's command history to the sandbox explicitly, so specs that do not
# set their own SQLY_HISTORY_DB_PATH still never touch the real history DB.
export SQLY_HISTORY_DB_PATH="$SANDBOX/history.db"

# Expose the sandbox root so the hermeticity scenarios can assert that HOME and
# the history DB live inside it.
export SQLY_E2E_SANDBOX="$SANDBOX"

# The freshly built sqly goes first on PATH so the specs exercise that exact binary.
PATH="$SANDBOX/bin:$PATH"
export PATH

# No `exec`: it would replace the shell and skip the EXIT trap, leaking the
# sandbox. As the last command under `set -e`, atago's exit status is the
# script's exit status either way.
atago run --ci "$@" "$ROOT/e2e/atago"
