package shell

import (
	"errors"
	"fmt"
	"os"
)

// What sqly does with a run is decided by three things: whether a query was
// given on the command line, whether stdin is a terminal, and whether stdin was
// claimed as data. Those combine into seven modes, and every question the rest
// of the program asks — is stdin read? as SQL or as data? may the script use
// helper commands? — is a property of the mode rather than a condition
// rediscovered at each call site.
//
// They were rediscovered at each call site before this: `!s.isTTY()` appeared in
// the batch branch, in the affected-count buffering, and in three validators,
// each deciding a piece of the same question. That is how a mode gets a
// half-defined corner, which is where the stdin hang lived.

// runMode is what this invocation of sqly is.
type runMode int

const (
	// modeInteractiveShell is the prompt: a terminal, no query flag.
	modeInteractiveShell runMode = iota
	// modeStdinScript reads a script from stdin: SQL statements and helper
	// commands, the same language typed at the prompt.
	modeStdinScript
	// modeInlineSQL runs the one statement given to --sql.
	modeInlineSQL
	// modeSQLFile runs the statements in a --sql-file. SQL only.
	modeSQLFile
	// modeInspect prints the JSON report and exits.
	modeInspect
)

// String names the mode for an error message.
func (m runMode) String() string {
	switch m {
	case modeInteractiveShell:
		return "interactive shell"
	case modeStdinScript:
		return "stdin script"
	case modeInlineSQL:
		return "--sql"
	case modeSQLFile:
		return "--sql-file"
	case modeInspect:
		return "--inspect"
	default:
		return "unknown"
	}
}

// readsStdinAsScript reports whether this mode reads stdin as SQL and helper
// commands. Exactly one mode does.
func (m runMode) readsStdinAsScript() bool { return m == modeStdinScript }

// allowsHelperCommands reports whether a script in this mode may contain helper
// commands. A script typed at the prompt or piped in is the shell's own
// language, so it may; a file named by --sql-file is a SQL file, and the flag
// says so.
func (m runMode) allowsHelperCommands() bool {
	return m == modeInteractiveShell || m == modeStdinScript
}

// runPlan is the decided shape of one invocation: the mode, and where stdin
// goes. It is computed once, before anything is imported.
type runPlan struct {
	mode runMode
	// stdinIsDataset is true when --stdin-format claimed stdin, so the mode's
	// script (if any) comes from somewhere else.
	stdinIsDataset bool
}

// planRun decides what this invocation is, and rejects the combinations that
// have no meaning. It reads no input: the decision is made from the flags and
// from what kind of thing stdin is, never from what stdin contains.
func (s *Shell) planRun() (runPlan, error) {
	arg := s.argument
	stdinIsDataset := arg.StdinFormat != ""

	// --sql and --sql-file both name the statements to run; two answers to one
	// question is a mistake, not a merge.
	if arg.Query != "" && arg.SQLFilePath != "" {
		return runPlan{}, &invocationError{Err: errors.New("--sql and --sql-file cannot be used together")}
	}

	switch {
	case arg.InspectFlag:
		return runPlan{mode: modeInspect, stdinIsDataset: stdinIsDataset}, nil
	case arg.Query != "":
		return s.planWithQuerySource(runPlan{mode: modeInlineSQL, stdinIsDataset: stdinIsDataset})
	case arg.SQLFilePath != "":
		return s.planWithQuerySource(runPlan{mode: modeSQLFile, stdinIsDataset: stdinIsDataset})
	}

	// No query flag: the script comes from stdin, or from the prompt.
	if stdinIsDataset {
		// stdin is the data, so nothing is left to carry the statements.
		return runPlan{}, &invocationError{Err: errors.New(
			"--stdin-format takes stdin as data, so there is no script to run; add --sql, --sql-file, or --inspect")}
	}
	if s.stdinKind() == stdinTerminal {
		return runPlan{mode: modeInteractiveShell}, nil
	}
	return runPlan{mode: modeStdinScript}, nil
}

// planWithQuerySource finishes a plan whose statements come from --sql or
// --sql-file, where stdin is either the dataset or unused.
//
// Unused is the case worth rejecting. A run given both a query flag and an input
// on stdin has an input the user meant something by; ignoring it and answering
// from the file arguments alone is the kind of silence that looks like success.
// So a stdin carrying something — a pipe, or a redirected file with bytes in it
// — that was not claimed by --stdin-format stops the run. A stdin that is known
// to be empty does not: nothing is being dropped.
func (s *Shell) planWithQuerySource(plan runPlan) (runPlan, error) {
	if plan.stdinIsDataset {
		return plan, nil
	}
	switch s.stdinKind() {
	case stdinPipe, stdinFile:
		return runPlan{}, &invocationError{Err: fmt.Errorf(
			"%s takes its statements from %s, so the data on stdin would be ignored; add --stdin-format FORMAT to read it as a table, or remove the redirect",
			plan.mode, plan.mode.source())}
	default:
		// A terminal, /dev/null, or an empty file: nothing was handed in that the
		// run would be throwing away. An empty stdin is how CI and wrappers invoke
		// a CLI they are not feeding, so it must stay valid.
		return plan, nil
	}
}

// source names where a mode's statements come from, for the message above.
func (m runMode) source() string {
	if m == modeSQLFile {
		return "the file"
	}
	return "the command line"
}

// stdinKind is what standard input is attached to. It is decided from the file
// descriptor alone — never by reading, and never by opening anything — because
// the one thing this must not do is block. A FIFO with no writer blocks on open
// and on read; a pipe with a live writer that has not written yet blocks on
// read. Both look identical to any check that tries to see whether data is
// there, which is why sqly does not try.
type stdinKind int

const (
	// stdinTerminal is an interactive terminal.
	stdinTerminal stdinKind = iota
	// stdinPipe is a pipe or FIFO: something is, or may be, feeding it.
	stdinPipe
	// stdinFile is a redirected regular file.
	stdinFile
	// stdinEmpty is a source that is known to carry nothing: /dev/null, or a
	// redirected file of zero length.
	stdinEmpty
	// stdinUnknown is anything else, including a descriptor that cannot be
	// examined. Treated as "may carry data" wherever that is the safe answer.
	stdinUnknown
)

// stdinKind classifies standard input. The result is cached: the answer cannot
// change during a run, and each probe is a syscall.
func (s *Shell) stdinKind() stdinKind {
	if s.stdinKindOnce {
		return s.stdinKindCached
	}
	s.stdinKindCached = s.probeStdin()
	s.stdinKindOnce = true
	return s.stdinKindCached
}

// probeStdin does the classification. isTTY is asked first because it is the
// only question a Stat cannot answer on every platform, and because a test
// Shell overrides it.
func (s *Shell) probeStdin() stdinKind {
	if s.isTTY != nil && s.isTTY() {
		return stdinTerminal
	}
	file, ok := s.stdin.(*os.File)
	if !ok {
		// A test or an embedder handed sqly a reader that is not a descriptor.
		// Nothing can be said about it without reading it, and unknown is the
		// answer that changes no behavior: production always has os.Stdin here.
		return stdinUnknown
	}
	info, err := file.Stat()
	if err != nil {
		return stdinUnknown
	}
	switch {
	case info.Mode()&os.ModeCharDevice != 0:
		// /dev/null and the other character devices. A terminal was handled above.
		return stdinEmpty
	case info.Mode()&os.ModeNamedPipe != 0:
		return stdinPipe
	case info.Mode().IsRegular():
		if info.Size() == 0 {
			return stdinEmpty
		}
		return stdinFile
	default:
		// Sockets, and anything a platform reports that does not fit above.
		return stdinPipe
	}
}
