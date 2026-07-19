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
	echo "e2e: e.g. 'go install github.com/nao1215/atago@v0.11.0' (CI uses nao1215/setup-atago)" >&2
	exit 127
fi

# Build the binary the specs exercise; it is exposed on PATH below.
#
# When COVER is set (by scripts/coverage.sh) the binary is built with Go's
# coverage instrumentation instead of the plain `make build`. atago passes the
# environment through to the spec commands, so the sqly child processes inherit
# GOCOVERDIR and write their runtime covdata there. The default (unset COVER)
# path stays byte-for-byte identical, keeping `make test-e2e` fast.
if [ -n "${COVER:-}" ]; then
	: "${GOCOVERDIR:?COVER set but GOCOVERDIR is empty; export GOCOVERDIR to collect e2e coverage}"
	# Mirror the Makefile's VERSION exactly (empty when no tags are reachable, e.g.
	# on a shallow CI checkout) so `sqly --version` resolves the same way the plain
	# `make build` binary does: an empty ldflag falls back to "(devel)".
	VERSION="$(git describe --tags --abbrev=0 2>/dev/null || true)"
	env GO111MODULE=on CGO_ENABLED=0 \
		go build -cover -covermode=atomic -coverpkg=./... \
		-ldflags "-X github.com/nao1215/sqly/config.Version=${VERSION}" \
		-o sqly main.go
else
	make build
fi

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
# sandbox. Under `set -e` a failing run stops the script, so a real regression in
# either pass surfaces; the last successful run's exit status is the script's.
#
# The interactive-shell pty specs (e2e/atago/pty.atago.yaml drives sqly's readline
# REPL over a pty) are split from the rest of the suite. The prompt is re-rendered a
# beat before its read loop is ready, so a keystroke sent right after can be lost
# when the pty sessions are starved of CPU by the other scenarios running in
# parallel. The rest of the suite runs in parallel; the pty specs then run on their
# own with --parallel 1 so each session gets uncontended CPU, and with extra
# retries. --retry-failed reports a recovered scenario as flaky, never hides it, and
# a real regression still fails after the retries.
PTY_SPEC="$ROOT/e2e/atago/pty.atago.yaml"

# Every spec except the pty one, collected so the parallel pass can skip it.
NON_PTY_SPECS=""
for spec in "$ROOT"/e2e/atago/*.atago.yaml; do
	[ "$spec" = "$PTY_SPEC" ] && continue
	NON_PTY_SPECS="$NON_PTY_SPECS $spec"
done

# shellcheck disable=SC2086 # intentional word splitting over the spec list
atago run --ci --retry-failed 3 "$@" $NON_PTY_SPECS
atago run --ci --parallel 1 --retry-failed 5 "$@" "$PTY_SPEC"
