package shell

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func hasSuggestionText(suggestions []Suggest, text string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Text == text {
			return true
		}
	}
	return false
}

// makeTree creates files and directories under the current working directory.
// A trailing slash marks a directory; everything else is created as an empty file.
func makeTree(t *testing.T, paths []string) {
	t.Helper()
	for _, p := range paths {
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(filepath.Clean(p), 0o750); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Clean(p))
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func completionTexts(suggestions []Suggest) []string {
	texts := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		texts = append(texts, s.Text)
	}
	return texts
}

func TestGetFilePathCompletionsScopedToTypedPrefix(t *testing.T) {
	// Note: cannot use t.Parallel() with t.Chdir().
	tmpDir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

	makeTree(t, []string{
		"top.csv",
		"testdata/actor.csv",
		"testdata/sample.tsv",
		"testdata/nested/deep.csv",
		"other/ignore.csv",
	})

	tests := []struct {
		name    string
		prefix  string
		want    []string
		notWant []string
	}{
		{
			name:    "empty prefix lists only the current directory, not nested files",
			prefix:  "",
			want:    []string{"top.csv", "testdata/", "other/"},
			notWant: []string{"testdata/actor.csv", "testdata/nested/deep.csv"},
		},
		{
			name:    "directory prefix lists entries inside that directory only",
			prefix:  "testdata/",
			want:    []string{"testdata/actor.csv", "testdata/sample.tsv", "testdata/nested/"},
			notWant: []string{"testdata/nested/deep.csv", "top.csv", "other/"},
		},
		{
			name:    "partial filename narrows to matching entries in the directory",
			prefix:  "testdata/ac",
			want:    []string{"testdata/actor.csv"},
			notWant: []string{"testdata/sample.tsv", "testdata/nested/"},
		},
		{
			name:    "partial directory name suggests the directory, not its nested files",
			prefix:  "testd",
			want:    []string{"testdata/"},
			notWant: []string{"testdata/actor.csv", "testdata/nested/deep.csv"},
		},
		{
			name:    "nested directory prefix scopes traversal to that subtree",
			prefix:  "testdata/nested/",
			want:    []string{"testdata/nested/deep.csv"},
			notWant: []string{"testdata/actor.csv", "top.csv"},
		},
		{
			// The directory must resolve even though the prefix uses a backslash;
			// suggestions keep the separator the user typed.
			name:    "backslash separator resolves the directory on every OS",
			prefix:  `testdata\`,
			want:    []string{"testdata/actor.csv", "testdata/sample.tsv"},
			notWant: []string{"top.csv"},
		},
	}

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// slash normalizes separators so the backslash case asserts the same way on
	// every OS (filepath.ToSlash rewrites "\" to "/" only on Windows).
	slash := func(s string) string { return strings.ReplaceAll(s, `\`, "/") }

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, text := range completionTexts(shell.getFilePathCompletions(tt.prefix)) {
				got = append(got, slash(text))
			}
			for _, w := range tt.want {
				if !slices.Contains(got, slash(w)) {
					t.Errorf("prefix %q: want completion %q, got %v", tt.prefix, w, got)
				}
			}
			for _, nw := range tt.notWant {
				if slices.Contains(got, slash(nw)) {
					t.Errorf("prefix %q: did not want completion %q, got %v", tt.prefix, nw, got)
				}
			}
		})
	}
}

func TestImportCompleterDebug(t *testing.T) {
	// Test the actual completer function behavior for .import commands
	tmpDir := t.TempDir()

	// Create test structure
	testStructure := map[string]bool{
		"testdata/actor.csv":  false,
		"testdata/sample.csv": false,
		"testdata/":           true,
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	shell, shellCleanup, shellErr := newShell(t, []string{"sqly"})
	if shellErr != nil {
		t.Fatal(shellErr)
	}
	defer shellCleanup()

	// Completion is scoped to the directory named by the typed prefix: a bare
	// ".import" or a partial directory name offers the directory, and only after
	// descending into it do the files inside appear.
	testCases := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "bare .import offers the top-level directory",
			text:     ".import ",
			expected: []string{"testdata/"},
		},
		{
			name:     "directory prefix lists the files inside it",
			text:     ".import testdata/",
			expected: []string{"testdata/actor.csv", "testdata/sample.csv"},
		},
		{
			name:     "partial directory name offers the directory",
			text:     ".import testd",
			expected: []string{"testdata/"},
		},
		{
			name:     "directory name without separator offers the directory",
			text:     ".import testdata",
			expected: []string{"testdata/"},
		},
		{
			name:     "partial filename narrows to the matching file",
			text:     ".import testdata/a",
			expected: []string{"testdata/actor.csv"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions := shell.getCompletions(context.Background(), tc.text)

			t.Logf("Input: '%s'", tc.text)
			for i, c := range completions {
				t.Logf("  %d: Text='%s', Desc='%s'", i, c.Text, c.Description)
			}

			for _, expected := range tc.expected {
				if !hasSuggestionText(completions, expected) {
					t.Errorf("Expected completion '%s' not found for input '%s'", expected, tc.text)
				}
			}
		})
	}
}

func TestCompleterDebug(t *testing.T) {
	// Test the actual completer function with mock document
	tmpDir := t.TempDir()

	// Create test structure
	testStructure := map[string]bool{
		"testdata/actor.csv":  false,
		"testdata/sample.csv": false,
		"testdata/":           true,
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	shell, shellCleanup, shellErr := newShell(t, []string{"sqly"})
	if shellErr != nil {
		t.Fatal(shellErr)
	}
	defer shellCleanup()

	// Test: Check if the problem is in isFilePath detection for various cases
	testCases := []struct {
		text         string
		expectedWord string
	}{
		{".import testdata", "testdata"},
		{".import testdata/", ""},   // GetWordBeforeCursor() returns empty after /
		{".import testdata/a", "a"}, // Only "a" after the last /
		{".import testdata/actor", "actor"},
	}

	for _, tc := range testCases {
		t.Logf("Testing text: '%s', expected currentWord: '%s'", tc.text, tc.expectedWord)

		// Simulate what GetWordBeforeCursor() would return
		currentWord := tc.expectedWord

		// Check isFilePath logic from completer function
		isFilePath := strings.Contains(currentWord, "/") ||
			strings.HasPrefix(currentWord, "./") ||
			strings.HasPrefix(currentWord, "../") ||
			strings.HasPrefix(currentWord, "~/") ||
			strings.HasPrefix(currentWord, "/") ||
			// Also check if the word looks like a filename with supported extensions
			(strings.Contains(currentWord, ".") &&
				(strings.Contains(currentWord, ".csv") ||
					strings.Contains(currentWord, ".tsv") ||
					strings.Contains(currentWord, ".ltsv") ||
					strings.Contains(currentWord, ".xlsx") ||
					strings.Contains(currentWord, ".gz") ||
					strings.Contains(currentWord, ".bz2") ||
					strings.Contains(currentWord, ".xz") ||
					strings.Contains(currentWord, ".zst")))

		atEndOfPath := strings.HasSuffix(tc.text, "/") && len(strings.TrimSpace(tc.text)) > 0

		t.Logf("Current word: '%s'", currentWord)
		t.Logf("isFilePath: %v", isFilePath)
		t.Logf("atEndOfPath: %v", atEndOfPath)

		// This should trigger .import command processing
		words := strings.Fields(tc.text)
		if len(words) >= 1 && words[0] == ".import" {
			t.Logf("Would trigger .import processing")
			fileCompletions := shell.getFilePathCompletions(tc.text)
			t.Logf("File completions: %d", len(fileCompletions))
			for i, c := range fileCompletions {
				t.Logf("  %d: Text='%s', Desc='%s'", i, c.Text, c.Description)
			}
		} else {
			t.Logf("Would NOT trigger .import processing")
		}
		t.Logf("") // Separator
	}

	// Test filterHasPrefix behavior
	suggestions := []Suggest{
		{Text: "testdata/", Description: "directory: testdata"},
	}

	filtered := filterHasPrefix(suggestions, "testdata", true)
	t.Logf("filterHasPrefix results with 'testdata': %d", len(filtered))
	for i, f := range filtered {
		t.Logf("  %d: Text='%s'", i, f.Text)
	}

	filtered2 := filterHasPrefix(suggestions, "testd", true)
	t.Logf("filterHasPrefix results with 'testd': %d", len(filtered2))
	for i, f := range filtered2 {
		t.Logf("  %d: Text='%s'", i, f.Text)
	}

	// Test filterHasPrefix with empty string
	actors := []Suggest{
		{Text: "actor.csv", Description: "file: actor.csv"},
		{Text: "sample.csv", Description: "file: sample.csv"},
	}

	filteredEmpty := filterHasPrefix(actors, "", true)
	t.Logf("filterHasPrefix with empty string: %d", len(filteredEmpty))

	filteredA := filterHasPrefix(actors, "a", true)
	t.Logf("filterHasPrefix with 'a': %d", len(filteredA))
	for i, f := range filteredA {
		t.Logf("  %d: Text='%s'", i, f.Text)
	}
}

func TestGoPromptCompletionBehavior(t *testing.T) {
	t.Logf("=== go-prompt TAB vs Arrow Key Behavior Analysis ===")

	// This test analyzes the difference between TAB and right arrow completion
	// Based on user observation: TAB shows candidates but doesn't progress,
	// right arrow applies completion and allows progression.

	// Theory: go-prompt expects specific Text format for proper TAB completion

	testCases := []struct {
		name           string
		input          string
		currentWord    string
		suggestionText string
		expectation    string
	}{
		{
			name:           "Directory completion with full path",
			input:          ".import testdata",
			currentWord:    "testdata",
			suggestionText: "testdata/", // Full replacement
			expectation:    "Should replace 'testdata' with 'testdata/' when TAB pressed",
		},
		{
			name:           "Directory completion with suffix only",
			input:          ".import testdata",
			currentWord:    "testdata",
			suggestionText: "/", // Only the missing suffix
			expectation:    "Should append '/' to 'testdata' when TAB pressed",
		},
	}

	for _, tc := range testCases {
		t.Logf("Case: %s", tc.name)
		t.Logf("  Input: %s", tc.input)
		t.Logf("  CurrentWord: %s", tc.currentWord)
		t.Logf("  SuggestionText: %s", tc.suggestionText)
		t.Logf("  Expectation: %s", tc.expectation)

		// Test filterHasPrefix behavior with different Text formats
		suggestions := []Suggest{
			{Text: tc.suggestionText, Description: "test completion"},
		}

		filtered := filterHasPrefix(suggestions, tc.currentWord, true)
		t.Logf("  FilterHasPrefix result: %d matches", len(filtered))
		if len(filtered) > 0 {
			t.Logf("    -> Text: '%s'", filtered[0].Text)
		}
		t.Logf("")
	}

	t.Logf("=== Key Insight ===")
	t.Logf("If TAB doesn't progress but right arrow does, the issue might be:")
	t.Logf("1. Text field format doesn't match go-prompt expectations")
	t.Logf("2. Completion isn't being 'committed' properly on TAB")
	t.Logf("3. Need to investigate go-prompt's internal TAB handling")

	t.Logf("")
	t.Logf("=== Real World Test Simulation ===")

	// Simulate the exact completer call for ".import testdata"
	tmpDir := t.TempDir()
	testStructure := map[string]bool{
		"testdata/actor.csv": false,
		"testdata/":          true,
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Logf("CONCLUSION:")
	t.Logf("Based on user feedback that TAB shows completions but right arrow applies them,")
	t.Logf("this suggests go-prompt's default TAB behavior is to show completions only.")
	t.Logf("The solution may require:")
	t.Logf("1. Setting OptionCompletionOnDown() - ✅ DONE")
	t.Logf("2. Ensuring completion uniqueness")
	t.Logf("3. Proper word separator configuration")
}

func TestRealDirectoryCompletion(t *testing.T) {
	// Exercise scoped completion against the real package directory, which has a
	// testdata/ subdirectory holding importable sample files.

	shell, shellCleanup, shellErr := newShell(t, []string{"sqly"})
	if shellErr != nil {
		t.Fatal(shellErr)
	}
	defer shellCleanup()

	t.Run("bare .import offers the testdata directory, not its files", func(t *testing.T) {
		got := completionTexts(shell.getFilePathCompletions(""))
		if !slices.Contains(got, "testdata/") {
			t.Errorf("expected testdata/ directory in completions, got %v", got)
		}
		if slices.Contains(got, "testdata/sample.csv") {
			t.Errorf("did not expect nested file testdata/sample.csv at the top level, got %v", got)
		}
	})

	t.Run("descending into testdata lists its importable files", func(t *testing.T) {
		completions := shell.getFilePathCompletions("testdata/")
		got := completionTexts(completions)
		if !slices.Contains(got, "testdata/sample.csv") {
			t.Errorf("expected testdata/sample.csv in completions, got %v", got)
		}
		for _, comp := range completions {
			if !strings.HasSuffix(comp.Text, "/") && comp.Description != msgImportableFile {
				t.Errorf("file %q has description %q, want %q", comp.Text, comp.Description, msgImportableFile)
			}
		}
	})

	t.Run("non-matching prefix yields no completions", func(t *testing.T) {
		if got := shell.getFilePathCompletions("nonexistent"); len(got) != 0 {
			t.Errorf("expected no completions for non-matching prefix, got %v", completionTexts(got))
		}
	})
}

func TestFilePathCompletions(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Chdir()

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files and directories
	testStructure := map[string]bool{
		"testdata/sample.csv":        false,
		"testdata/sample.tsv":        false,
		"testdata/sample.ltsv":       false,
		"testdata/sample.xlsx":       false,
		"testdata/compressed.csv.gz": false,
		"testdata/":                  true,
		"docs/":                      true,
		"docs/readme.md":             false,
		"config.yaml":                false,
	}

	// Change to temp directory - using t.Chdir for Go 1.20+
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

	// Create the directory structure
	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0o750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			f, err := os.Create(filepath.Clean(path))
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}

	// Create shell instance
	shell, shellCleanup, shellErr := newShell(t, []string{"sqly"})
	if shellErr != nil {
		t.Fatal(shellErr)
	}
	defer shellCleanup()

	tests := []struct {
		name     string
		input    string
		expected []string
		excluded []string
	}{
		{
			name:     "empty prefix lists top-level directories only",
			input:    "",
			expected: []string{"testdata/", "docs/"},
			excluded: []string{"testdata/sample.csv", "config.yaml"},
		},
		{
			name:  "directory prefix lists importable files inside it",
			input: "testdata/",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
				"testdata/compressed.csv.gz",
			},
		},
		{
			name:  "partial filename narrows to matching files",
			input: "testdata/sample",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
			},
			excluded: []string{"testdata/compressed.csv.gz"},
		},
		{
			name:     "partial directory name offers the directory only",
			input:    "test",
			expected: []string{"testdata/"},
			excluded: []string{"testdata/sample.csv"},
		},
		{
			name:     "directory without importable files yields no file suggestions",
			input:    "docs/",
			excluded: []string{"docs/readme.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := completionTexts(shell.getFilePathCompletions(tt.input))
			t.Logf("Input %q -> %v", tt.input, results)

			for _, expected := range tt.expected {
				if !slices.Contains(results, expected) {
					t.Errorf("Expected completion '%s' not found in results: %v", expected, results)
				}
			}
			for _, excluded := range tt.excluded {
				if slices.Contains(results, excluded) {
					t.Errorf("Did not expect completion '%s' in results: %v", excluded, results)
				}
			}
		})
	}
}

// Skip integration test for now due to prompt.Document complexity
// The file path completion logic is tested separately

func TestIsValidFileForCompletion(t *testing.T) {
	t.Parallel()

	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	tests := []struct {
		filename string
		expected bool
	}{
		{"sample.csv", true},
		{"sample.tsv", true},
		{"sample.ltsv", true},
		{"sample.xlsx", true},
		{"sample.json", true},
		{"sample.jsonl", true},
		{"sample.parquet", true},
		{"sample.csv.gz", true},
		{"sample.tsv.bz2", true},
		{"sample.ltsv.xz", true},
		{"sample.xlsx.zst", true},
		{"sample.csv.snappy", true},
		{"sample.csv.s2", true},
		{"sample.csv.lz4", true},
		{"sample.csv.z", true},
		{"sample.txt", false},
		{"sample", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := s.isValidFileForCompletion(tt.filename)
			if result != tt.expected {
				t.Errorf("isValidFileForCompletion(%s) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestCompleterIntegration(t *testing.T) {
	// Test the actual completer method (integration test) with new full-path completion

	// Create a shell instance
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	testCases := []struct {
		name     string
		input    string
		wantDir  bool // expect at least one directory suggestion
		wantFile bool // expect at least one importable-file suggestion
		wantText string
	}{
		{
			name:     "bare .import offers the testdata directory",
			input:    ".import ",
			wantDir:  true,
			wantText: "testdata/",
		},
		{
			name:     "partial directory name offers the directory",
			input:    ".import testd",
			wantDir:  true,
			wantText: "testdata/",
		},
		{
			name:     "descending into testdata offers importable files",
			input:    ".import testdata/",
			wantFile: true,
			wantText: "testdata/sample.csv",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions := shell.getCompletions(context.Background(), tc.input)

			t.Logf("Input: '%s'", tc.input)
			for i, c := range completions {
				t.Logf("  %d: '%s' - %s", i, c.Text, c.Description)
			}

			if !hasSuggestionText(completions, tc.wantText) {
				t.Errorf("expected completion %q for input %q", tc.wantText, tc.input)
			}

			hasDir, hasFile := false, false
			for _, comp := range completions {
				switch comp.Description {
				case msgImportableDir:
					hasDir = true
				case msgImportableFile:
					hasFile = true
				}
			}
			if tc.wantDir && !hasDir {
				t.Error("expected at least one directory suggestion")
			}
			if tc.wantFile && !hasFile {
				t.Error("expected at least one importable file suggestion")
			}
		})
	}
}

func TestCompleterNonImportCommands(t *testing.T) {
	t.Parallel()

	// Test completer with non-import commands
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Import some data to test table completions
	testdataPath := "testdata"
	if _, err := os.Stat(testdataPath); os.IsNotExist(err) {
		testdataPath = "../testdata"
	}
	if err := shell.commands.importCommand(context.Background(), shell, []string{filepath.Join(testdataPath, "sample.csv")}); err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		name         string
		input        string
		expectFiles  bool
		expectTables bool
	}{
		{
			name:         "Help command should not show file completions",
			input:        ".help",
			expectFiles:  false,
			expectTables: false,
		},
		{
			name:         "Tables command should not show file completions",
			input:        ".tables",
			expectFiles:  false,
			expectTables: false,
		},
		{
			name:         "SQL query should not show file completions",
			input:        "SELECT * FROM ",
			expectFiles:  false,
			expectTables: true,
		},
		{
			name:         "Empty input should show table completions",
			input:        "",
			expectFiles:  false,
			expectTables: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions := shell.getCompletions(context.Background(), tc.input)

			t.Logf("Input: '%s'", tc.input)
			t.Logf("Got %d completions:", len(completions))

			hasFiles := false
			hasTables := false
			for _, comp := range completions {
				if comp.Description == "Importable file" {
					hasFiles = true
				}
				if strings.HasPrefix(comp.Description, "table: ") {
					hasTables = true
				}
			}

			if tc.expectFiles && !hasFiles {
				t.Error("Expected file completions but found none")
			}
			if !tc.expectFiles && hasFiles {
				t.Error("Did not expect file completions but found some")
			}
			if tc.expectTables && !hasTables {
				t.Error("Expected table completions but found none")
			}
		})
	}
}

func TestShell_getRegularCompletions_dependsOnMetadataUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)

	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{
		model.NewTable("users", nil, nil),
	}, nil)
	metadata.EXPECT().Header(gomock.Any(), "users").Return(
		model.NewTable("users", model.NewHeader([]string{"id", "name"}), nil), nil)

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})

	completions := s.getRegularCompletions(context.Background(), "")
	if !hasSuggestionText(completions, "users") {
		t.Fatalf("completions do not include table suggestion: %#v", completions)
	}
	if !hasSuggestionText(completions, "name") {
		t.Fatalf("completions do not include header suggestion: %#v", completions)
	}
}

func TestShell_getFilePathCompletions_dependsOnImportUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	for _, file := range []string{"data.csv", "notes.txt", ".hidden.csv"} {
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", file, err)
		}
	}

	importer.EXPECT().IsSupportedFile("data.csv").Return(true)
	importer.EXPECT().IsSupportedFile("notes.txt").Return(false)

	s := newBoundaryTestShell(t, Usecases{importer: importer})

	completions := s.getFilePathCompletions("")
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d: %#v", len(completions), completions)
	}
	if completions[0].Text != "data.csv" {
		t.Fatalf("completion text = %q, want %q", completions[0].Text, "data.csv")
	}
	if completions[0].Description != msgImportableFile {
		t.Fatalf("completion description = %q, want %q", completions[0].Description, msgImportableFile)
	}
}

func TestCompleterEdgeCases(t *testing.T) {
	t.Parallel()

	// Test completer edge cases
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "Import command with trailing space",
			input: ".import ",
		},
		{
			name:  "Import command without space",
			input: ".import",
		},
		{
			name:  "Import command with multiple spaces",
			input: ".import   ",
		},
		{
			name:  "Import with path separator",
			input: ".import /",
		},
		{
			name:  "Import with current directory",
			input: ".import ./",
		},
		{
			name:  "Import with parent directory",
			input: ".import ../",
		},
		{
			name:  "Import with home directory",
			input: ".import ~/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions := shell.getCompletions(context.Background(), tc.input)

			t.Logf("Input: '%s' -> %d completions", tc.input, len(completions))

			// Should not panic and should return some result
			// Just verify we got a valid response (len cannot be negative)
		})
	}
}

// BenchmarkGetFilePathCompletions measures completion against a large synthetic
// directory tree. The fix scopes traversal to the targeted directory, so latency
// should track that single directory rather than the whole tree. The benchmark
// compares completing inside one leaf directory against listing the shallow root,
// making any regression to whole-tree scanning measurable.
func BenchmarkGetFilePathCompletions(b *testing.B) {
	b.Chdir(b.TempDir())

	// Build a wide and deep tree: 50 top-level directories, each with 50 files.
	const dirs, filesPerDir = 50, 50
	for d := range dirs {
		dir := filepath.Join("data", "dir"+strconv.Itoa(d))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			b.Fatal(err)
		}
		for f := range filesPerDir {
			name := filepath.Join(dir, "file"+strconv.Itoa(f)+".csv")
			if err := os.WriteFile(name, []byte("a\n"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}

	shell, cleanup, err := newShell(b, []string{"sqly"})
	if err != nil {
		b.Fatal(err)
	}
	defer cleanup()

	leaf := filepath.ToSlash(filepath.Join("data", "dir25")) + "/"

	b.Run("leaf directory", func(b *testing.B) {
		for b.Loop() {
			_ = shell.getFilePathCompletions(leaf)
		}
	})
	b.Run("root directory", func(b *testing.B) {
		for b.Loop() {
			_ = shell.getFilePathCompletions("")
		}
	})
}
