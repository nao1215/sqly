package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestDescribeFileModeNamesEachKind checks the word a refused --output names
// the thing in the way with, for each kind of non-regular file.
func TestDescribeFileModeNamesEachKind(t *testing.T) {
	t.Parallel()

	for mode, want := range map[os.FileMode]string{
		os.ModeNamedPipe:                    "named pipe",
		os.ModeSocket:                       "socket",
		os.ModeDevice | os.ModeCharDevice:   "character device",
		os.ModeDevice:                       "device",
		os.ModeIrregular:                    "special file",
		os.ModeNamedPipe | os.ModeIrregular: "named pipe",
	} {
		if got := describeFileMode(mode); got != want {
			t.Errorf("describeFileMode(%v) = %q, want %q", mode, got, want)
		}
	}
}

// TestOutputRefusesAParentThatIsAFile checks that --output under a path whose
// parent is a regular file is refused before anything runs, naming the parent.
func TestOutputRefusesAParentThatIsAFile(t *testing.T) {
	dir := t.TempDir()
	src := writeCSV(t, dir, "user.csv", "id\n1\n")
	dest := filepath.Join(src, "out.csv")

	_, _, err := runWithArgs(t, "--sql", "SELECT * FROM user", "--output", dest, src)
	if err == nil || !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("--output %s = %v, want an error saying the parent is not a directory", dest, err)
	}
}

// TestOutputFormatDescriptionSaysWhatTheFormatDoes checks the completion text
// of the formats whose name alone does not say what they produce, and of one
// that does.
func TestOutputFormatDescriptionSaysWhatTheFormatDoes(t *testing.T) {
	t.Parallel()

	for mode, want := range map[model.PrintMode]string{
		model.PrintModeMarkdownTable: "markdown table output format",
		model.PrintModeJSONL:         "jsonl (newline-delimited JSON) output format",
		model.PrintModeParquet:       "parquet export format",
		model.PrintModeCSV:           "csv output format",
	} {
		if got := outputFormatDescription(mode); got != want {
			t.Errorf("outputFormatDescription(%v) = %q, want %q", mode, got, want)
		}
	}
}
