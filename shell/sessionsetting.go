package shell

import (
	"fmt"

	"github.com/nao1215/sqly/config"
)

// The session settings are .dialect, .mode, and .row-mismatch. Each names one
// property of the session, each is written the same way (".CMD VALUE"), and
// each is read the same way (".CMD" alone). This file is where the shape of
// that answer is written down once, so the three cannot answer differently.
//
// Why an argument-less call reports instead of failing: ".mode" and
// ".row-mismatch" used to be errors, on the reasoning that a script meaning
// ".mode csv" should not continue silently in the wrong mode. The typo it
// guards against is still caught — ".mode csvv" is rejected by name — and what
// the guard actually caught was a person asking a question. sqlite3 answers
// that question; three commands of one family answering it three ways is worse
// than the case the error was for.
//
// Why stderr: stdout carries the rows a program parses. A control line printed
// there breaks `sqly --output-format json --script-file s.sqly | jq .` for
// anyone whose script happens to name its dialect, and no format is safe from
// it — csv, tsv, ltsv, and jsonl all lose the same way. Nothing about a setting
// is data, so nothing about it reaches stdout.

// printSessionSetting reports one session setting: "current <setting>: <value>
// (available: <values>)". available is the comma-separated list of accepted
// values, taken from whichever registry owns them.
func printSessionSetting(setting, current, available string) {
	fmt.Fprintf(config.Stderr, "current %s: %s (available: %s)\n", setting, current, available)
}

// The setting names printSessionSetting prints, spelled once so the three
// commands and their tests read the same words.
const (
	settingDialect     = "dialect"
	settingOutputMode  = "output mode"
	settingRowMismatch = "row-mismatch policy"
)
