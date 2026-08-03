package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nao1215/sqly/config"
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
		return s.planWithQuerySource(runPlan{mode: modeInspect, stdinIsDataset: stdinIsDataset})
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

// planWithQuerySource finishes a plan whose work comes from --sql, --sql-file, or
// --inspect, where stdin is either the dataset or unused.
//
// Unused is the case worth saying something about. `cat data.csv | sqly --sql
// "..." other.csv` looks like it feeds data.csv in, and it does not: the answer
// comes from other.csv alone and looks perfectly correct. So sqly says so.
//
// It says so rather than failing, because it cannot tell a pipe carrying a file
// from a pipe carrying nothing without reading it, and reading it is what hangs
// on a FIFO with no writer. A wrapper that attaches an empty pipe to every child
// — Go's os/exec does, and so do several CI runners — must keep working, so the
// exit code does not change and only stderr does.
func (s *Shell) planWithQuerySource(plan runPlan) (runPlan, error) {
	if plan.stdinIsDataset || s.stdinNamedAsInput() {
		return plan, nil
	}
	switch s.stdinKind() {
	case stdinPipe, stdinFile:
		fmt.Fprintf(config.Stderr,
			"warning: standard input was not read; %s works from %s. To query it, add --stdin-format FORMAT.\n",
			plan.mode, plan.mode.source())
	default:
		// A terminal, /dev/null, or an empty file: nothing was handed in that the
		// run would be throwing away, so there is nothing to mention.
	}
	return plan, nil
}

// stdinNamedAsInput reports whether an input argument is standard input under one
// of the names a shell gives it. `sqly --sql "..." /dev/stdin` reads the pipe as a
// file, so the pipe is not being dropped and the rejection above must not fire —
// the user said what to do with stdin, just not with a flag.
func (s *Shell) stdinNamedAsInput() bool {
	for _, path := range s.argument.FilePaths {
		switch filepath.ToSlash(path) {
		case "/dev/stdin", "/dev/fd/0", "/proc/self/fd/0":
			return true
		}
	}
	return false
}

// source names where a mode's work comes from, for the message above.
func (m runMode) source() string {
	switch m {
	case modeSQLFile:
		return "the statements in the file"
	case modeInspect:
		return "the files named on the command line"
	default:
		return "the statement on the command line"
	}
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
