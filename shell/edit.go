package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
)

// editorEnvVars are the environment variables that name the editor, in the
// order every tool that opens one consults them. VISUAL is the full-screen
// editor and EDITOR the line editor, a distinction that stopped mattering long
// ago; what is left is the convention that VISUAL wins where both are set.
var editorEnvVars = []string{"VISUAL", "EDITOR"}

// editedFilePattern is the temp file the editor is given. The .sql suffix is
// what makes an editor highlight the buffer as SQL, which is most of the point
// of leaving the prompt for it.
const editedFilePattern = "sqly-edit-*.sql"

// isEditRequest reports whether input is the .edit command and, when it is,
// whether it was written correctly. Arguments are rejected here rather than in
// the command table because the interactive loop takes this line before the
// table sees it: .edit needs the prompt session, which only the loop holds.
func isEditRequest(input string) (bool, error) {
	argv, err := splitArgs(input)
	if err != nil {
		// A line sqly cannot even split into words is not this command. The
		// dispatcher reports the quoting error, which says more than a refusal
		// naming .edit would.
		return false, nil //nolint:nilerr // an unsplittable line is not .edit
	}
	if len(argv) == 0 || argv[0] != editCommand {
		return false, nil
	}
	if len(argv) > 1 {
		return true, &invocationError{Err: fmt.Errorf("%s takes no arguments, got %d; it opens the last statement, and what the editor saves is what runs", editCommand, len(argv)-1)}
	}
	return true, nil
}

// runEditor opens seed in the user's editor and returns what was saved, ready
// to run. It returns "" when nothing should run: the file came back holding no
// statement.
//
// The caller closes the prompt session first and opens a fresh one afterwards.
// That is not tidiness. A prompt owns the terminal while it lives -- raw mode,
// and a goroutine reading the terminal -- so an editor started underneath it
// would draw on a terminal it does not control and lose its keystrokes to sqly.
func (s *Shell) runEditor(ctx context.Context, seed string) (string, error) {
	editor, err := s.editorCommand()
	if err != nil {
		return "", err
	}

	path, cleanup, err := writeEditedFile(seed)
	if err != nil {
		return "", err
	}
	defer cleanup()

	if err := s.editorRunner(ctx, editor, path); err != nil {
		return "", err
	}

	edited, err := os.ReadFile(path) //nolint:gosec // the path is the temp file written just above
	if err != nil {
		return "", fmt.Errorf("failed to read back what the editor saved: %w", err)
	}
	return strings.TrimSpace(string(edited)), nil
}

// editorCommand returns the editor to run, split into a command and its
// arguments. It is split with the same quoting rules a helper command's
// arguments use, so an editor set with flags ("code -w") and one whose path
// holds a space work alike.
func (s *Shell) editorCommand() ([]string, error) {
	for _, name := range editorEnvVars {
		value := strings.TrimSpace(s.getenv(name))
		if value == "" {
			continue
		}
		argv, err := splitArgs(value)
		if err != nil {
			return nil, &invocationError{Err: fmt.Errorf("%s=%q cannot be read as a command: %w", name, value, err)}
		}
		if len(argv) == 0 {
			continue
		}
		return argv, nil
	}
	return nil, &invocationError{Err: fmt.Errorf("no editor is set; %s opens $VISUAL or $EDITOR, e.g. EDITOR=vim sqly file.csv", editCommand)}
}

// writeEditedFile puts seed in a temp file for the editor and returns its path
// with the function that removes it. The file is created in a directory of its
// own, which is what goes at the end: an editor that saves through a temp file
// or leaves a swap file behind puts it beside ours, and a directory takes them
// all with it.
func writeEditedFile(seed string) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "sqly-edit")
	if err != nil {
		return "", nil, fmt.Errorf("failed to make a place for the editor to write: %w", err)
	}
	cleanup = func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(config.Stderr, "warning: failed to remove the edit scratch directory %s: %v\n", dir, err)
		}
	}

	file, err := os.CreateTemp(dir, editedFilePattern)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to make a file for the editor: %w", err)
	}
	path = file.Name()

	// The seed ends with a newline so the editor opens on a line of its own
	// below it, which is where the next line of the statement is written.
	contents := seed
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	if _, err := file.WriteString(contents); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, fmt.Errorf("failed to write the statement for the editor: %w", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close the file for the editor: %w", err)
	}
	return path, cleanup, nil
}

// runEditorProcess runs the editor over path with the terminal attached, so a
// full-screen editor can draw on it and read from it.
//
// A non-zero exit is how an editor says the edit was abandoned (":cq" in vim),
// so it is reported as such and nothing runs. The alternative -- running
// whatever happens to be in the file -- is the one outcome a user who
// deliberately aborted did not ask for.
func runEditorProcess(ctx context.Context, argv []string, path string) error {
	// The session's context, so an editor is not orphaned by a sqly that is
	// itself being shut down.
	cmd := exec.CommandContext(ctx, argv[0], append(argv[1:], path)...) //nolint:gosec // the command is the editor the user chose
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%s exited with status %d; nothing was run", filepath.Base(argv[0]), exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run the editor %s: %w", argv[0], err)
	}
	return nil
}

// editCommand is what .edit reports when it is reached outside the interactive
// loop. The command is in the table so .help lists it, completion offers it,
// and a mistyped name can be corrected to it; running it is the loop's job,
// because opening an editor means closing the prompt session and opening
// another, and only the loop holds one. A script has no terminal to hand over.
func (c CommandList) editCommand(_ context.Context, _ *Shell, _ []string) error {
	return &invocationError{Err: fmt.Errorf("%s needs an interactive terminal; write the SQL in a file and run it with --sql-file, or pipe it to sqly", editCommand)}
}
