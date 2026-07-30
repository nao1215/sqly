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
	echo "e2e: e.g. 'go install github.com/nao1215/atago@v0.18.0' (CI uses nao1215/setup-atago)" >&2
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
# retries.
#
# Only the pty pass retries, and only it accepts a flaky verdict. As of atago
# v0.18.0 a recovered scenario fails the run unless --allow-flaky says the
# instability is expected, which is exactly the split wanted here: the pty
# sessions lose keystrokes under CPU starvation and that is known, while the
# non-pty specs assert fixed output from a subprocess and have no timing to lose.
#
# Before that split, one --retry-failed covered the whole suite and quietly
# extended tolerance to every spec: filesql built an LTSV table's column list by
# ranging over a map, so `SELECT *` answered in a different order on every run,
# and the retries turned that into "flaky, PASSED" instead of a failure.
PTY_SPEC="$ROOT/e2e/atago/pty.atago.yaml"

# Every spec except the pty one, collected so the parallel pass can skip it.
NON_PTY_SPECS=""
for spec in "$ROOT"/e2e/atago/*.atago.yaml; do
	[ "$spec" = "$PTY_SPEC" ] && continue
	NON_PTY_SPECS="$NON_PTY_SPECS $spec"
done

# When a focused `--filter` selects only non-PTY scenarios (or only PTY ones),
# atago's CI mode rejects the empty second pass as a silent suite disable. Check
# the selected scenario names up front and skip passes that cannot match the
# requested filter, while preserving atago's native "no scenarios matched"
# failure when the filter misses the whole suite.
FILTER_PATTERNS=""
append_filter_patterns() {
	if [ -z "$FILTER_PATTERNS" ]; then
		FILTER_PATTERNS=$1
	else
		FILTER_PATTERNS="$FILTER_PATTERNS,$1"
	fi
}

collect_filter_patterns() {
	while [ "$#" -gt 0 ]; do
		case "$1" in
			--filter)
				shift
				[ "$#" -gt 0 ] || break
				append_filter_patterns "$1"
				;;
			--filter=*)
				append_filter_patterns "${1#--filter=}"
				;;
		esac
		shift
	done
}

spec_matches_filter() {
	spec=$1
	[ -z "$FILTER_PATTERNS" ] && return 0
	names="$(sed -n \
		-e 's/^[[:space:]]*-[[:space:]]*name:[[:space:]]*"\(.*\)"[[:space:]]*$/\1/p' \
		-e "s/^[[:space:]]*-[[:space:]]*name:[[:space:]]*'\\(.*\\)'[[:space:]]*$/\\1/p" \
		-e 's/^[[:space:]]*-[[:space:]]*name:[[:space:]]*\([^"'"'"'"'"'"'"'"'"'].*\)$/\1/p' \
		"$spec")"
	old_ifs=$IFS
	IFS=','
	for needle in $FILTER_PATTERNS; do
		case "$names" in
			*"$needle"*)
				IFS=$old_ifs
				return 0
				;;
		esac
	done
	IFS=$old_ifs
	return 1
}

spec_group_matches_filter() {
	[ "$#" -gt 0 ] || return 1
	[ -z "$FILTER_PATTERNS" ] && return 0
	for spec in "$@"; do
		if spec_matches_filter "$spec"; then
			return 0
		fi
	done
	return 1
}

collect_filter_patterns "$@"

ran_any=false

# shellcheck disable=SC2086 # intentional word splitting over the spec list
if spec_group_matches_filter $NON_PTY_SPECS; then
	ran_any=true
	atago run --ci --retry-failed 0 "$@" $NON_PTY_SPECS
fi
if spec_group_matches_filter "$PTY_SPEC"; then
	ran_any=true
	atago run --ci --parallel 1 --retry-failed 5 --allow-flaky "$@" "$PTY_SPEC"
fi
if [ "$ran_any" = false ]; then
	# Neither group matched, so nothing was selected; run everything and let atago
	# report the empty selection. The pty retry count applies because this pass
	# includes the pty spec.
	atago run --ci --parallel 1 --retry-failed 5 --allow-flaky "$@" $NON_PTY_SPECS "$PTY_SPEC"
fi
