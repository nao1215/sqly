package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c-bata/go-prompt"
	"github.com/nao1215/sqly/config"
)

func TestImportCompleterDebug(t *testing.T) {
	// Test the actual completer function behavior for .import commands
	tmpDir := t.TempDir()

	// Create test structure
	testStructure := map[string]bool{
		"testdata/actor.csv":  false,
		"testdata/sample.csv": false,
		"testdata/":           true,
	}

	t.Chdir(tmpDir)

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
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

	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	// Test different states of completion
	testCases := []struct {
		name              string
		text              string
		expectedFilenames []string
	}{
		{
			name:              "After .import with space",
			text:              ".import ",
			expectedFilenames: []string{"testdata/"},
		},
		{
			name:              "After .import testdata/",
			text:              ".import testdata/",
			expectedFilenames: []string{"actor.csv", "sample.csv"},
		},
		{
			name:              "After partial .import testd",
			text:              ".import testd",
			expectedFilenames: []string{"testdata/"},
		},
		{
			name:              "After partial input testdata (without slash)",
			text:              ".import testdata",
			expectedFilenames: []string{"testdata/"},
		},
		{
			name:              "After testdata/ + partial filename 'a'",
			text:              ".import testdata/a",
			expectedFilenames: []string{"actor.csv"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fileCompletions := shell.getFilePathCompletions(tc.text)

			t.Logf("Input: '%s'", tc.text)
			t.Logf("Completions: %d", len(fileCompletions))
			for i, c := range fileCompletions {
				t.Logf("  %d: Text='%s', Desc='%s'", i, c.Text, c.Description)
			}

			// Verify expected files are present
			for _, expectedFile := range tc.expectedFilenames {
				found := false
				for _, completion := range fileCompletions {
					if completion.Text == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completion '%s' not found in results for input '%s'", expectedFile, tc.text)
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

	t.Chdir(tmpDir)

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
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

	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

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

	// Test FilterHasPrefix behavior
	suggestions := []prompt.Suggest{
		{Text: "testdata/", Description: "directory: testdata"},
	}

	filtered := prompt.FilterHasPrefix(suggestions, "testdata", true)
	t.Logf("FilterHasPrefix results with 'testdata': %d", len(filtered))
	for i, f := range filtered {
		t.Logf("  %d: Text='%s'", i, f.Text)
	}

	filtered2 := prompt.FilterHasPrefix(suggestions, "testd", true)
	t.Logf("FilterHasPrefix results with 'testd': %d", len(filtered2))
	for i, f := range filtered2 {
		t.Logf("  %d: Text='%s'", i, f.Text)
	}

	// Test FilterHasPrefix with empty string
	actors := []prompt.Suggest{
		{Text: "actor.csv", Description: "file: actor.csv"},
		{Text: "sample.csv", Description: "file: sample.csv"},
	}

	filteredEmpty := prompt.FilterHasPrefix(actors, "", true)
	t.Logf("FilterHasPrefix with empty string: %d", len(filteredEmpty))

	filteredA := prompt.FilterHasPrefix(actors, "a", true)
	t.Logf("FilterHasPrefix with 'a': %d", len(filteredA))
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

		// Test FilterHasPrefix behavior with different Text formats
		suggestions := []prompt.Suggest{
			{Text: tc.suggestionText, Description: "test completion"},
		}

		filtered := prompt.FilterHasPrefix(suggestions, tc.currentWord, true)
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
	t.Chdir(tmpDir)

	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
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
	// Test with actual directory structure to identify the golden/testdata issue

	// Change to project root directory so we can find both testdata and golden directories
	t.Chdir("..")

	// Create a shell instance
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	// Test cases based on real directory structure
	testCases := []struct {
		name             string
		input            string
		shouldContain    []string
		shouldNOTContain []string
	}{
		{
			name:             "Empty input should show directories only",
			input:            ".import ",
			shouldContain:    []string{"testdata/", "golden/"},
			shouldNOTContain: []string{"main.go", "README.md"},
		},
		{
			name:             "testdata prefix should only show testdata",
			input:            ".import testdata",
			shouldContain:    []string{"testdata/"},
			shouldNOTContain: []string{"golden/"}, // This is the key issue!
		},
		{
			name:             "golden prefix should only show golden",
			input:            ".import golden",
			shouldContain:    []string{"golden/"},
			shouldNOTContain: []string{"testdata/"},
		},
		{
			name:             "non-existent prefix should show nothing",
			input:            ".import nonexistent",
			shouldContain:    []string{},
			shouldNOTContain: []string{"testdata/", "golden/"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions := shell.getFilePathCompletions(tc.input)

			t.Logf("Input: '%s'", tc.input)
			t.Logf("Got %d completions:", len(completions))
			for i, c := range completions {
				t.Logf("  %d: '%s' - %s", i, c.Text, c.Description)
			}

			// Check should contain
			for _, expected := range tc.shouldContain {
				found := false
				for _, comp := range completions {
					if comp.Text == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find '%s' in completions", expected)
				}
			}

			// Check should NOT contain
			for _, notExpected := range tc.shouldNOTContain {
				for _, comp := range completions {
					if comp.Text == notExpected {
						t.Errorf("Should NOT find '%s' in completions but found it", notExpected)
					}
				}
			}
		})
	}
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
	t.Chdir(tmpDir)

	// Create the directory structure
	for path, isDir := range testStructure {
		if isDir {
			if err := os.MkdirAll(path, 0750); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
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
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Complete root level directories",
			input:    "",
			expected: []string{"testdata/", "docs/"},
		},
		{
			name:  "Complete testdata directory",
			input: "testdata/",
			expected: []string{
				"sample.csv",
				"sample.tsv",
				"sample.ltsv",
				"sample.xlsx",
				"compressed.csv.gz",
			},
		},
		{
			name:  "Complete with partial filename",
			input: "testdata/sample",
			expected: []string{
				"sample.csv",
				"sample.tsv",
				"sample.ltsv",
				"sample.xlsx",
			},
		},
		{
			name:     "Complete with partial directory",
			input:    "test",
			expected: []string{"testdata/"},
		},
		{
			name:  "Complete after .import command with directory",
			input: ".import testdata/",
			expected: []string{
				"sample.csv",
				"sample.tsv",
				"sample.ltsv",
				"sample.xlsx",
				"compressed.csv.gz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions := shell.getFilePathCompletions(tt.input)

			// Extract completion texts
			var results []string
			for _, c := range completions {
				results = append(results, c.Text)
			}

			// Check if all expected completions are present
			for _, expected := range tt.expected {
				found := false
				for _, result := range results {
					if result == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected completion '%s' not found in results: %v", expected, results)
				}
			}
		})
	}
}

// Skip integration test for now due to prompt.Document complexity
// The file path completion logic is tested separately

func TestSupportedFileExtensions(t *testing.T) {
	t.Parallel()

	extensions := supportedFileExtensions()
	expected := []string{".csv", ".tsv", ".ltsv", ".xlsx"}

	if len(extensions) != len(expected) {
		t.Errorf("Expected %d extensions, got %d", len(expected), len(extensions))
	}

	for _, ext := range expected {
		found := false
		for _, actual := range extensions {
			if actual == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected extension '%s' not found", ext)
		}
	}
}

func TestIsValidFileForCompletion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		expected bool
	}{
		{"sample.csv", true},
		{"sample.tsv", true},
		{"sample.ltsv", true},
		{"sample.xlsx", true},
		{"sample.csv.gz", true},
		{"sample.tsv.bz2", true},
		{"sample.ltsv.xz", true},
		{"sample.xlsx.zst", true},
		{"sample.txt", false},
		{"sample.json", false},
		{"sample", false},
		{"README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isValidFileForCompletion(tt.filename)
			if result != tt.expected {
				t.Errorf("isValidFileForCompletion(%s) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestCompleterIntegration(t *testing.T) {
	// Test the actual completer method (integration test)

	// Change to project root directory so we can find both testdata and golden directories
	t.Chdir("..")

	// Create a shell instance
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	testCases := []struct {
		name             string
		input            string
		shouldContain    []string
		shouldNOTContain []string
	}{
		{
			name:             "Empty input should show directories and regular completions",
			input:            ".import ",
			shouldContain:    []string{"testdata/", "golden/"},
			shouldNOTContain: []string{},
		},
		{
			name:             "testdata prefix should only show testdata",
			input:            ".import testdata",
			shouldContain:    []string{"testdata/"},
			shouldNOTContain: []string{"golden/"},
		},
		{
			name:             "golden prefix should only show golden",
			input:            ".import golden",
			shouldContain:    []string{"golden/"},
			shouldNOTContain: []string{"testdata/"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create document for the completer
			doc := prompt.Document{Text: tc.input}
			completions := shell.completer(context.Background(), doc)

			t.Logf("Input: '%s'", tc.input)
			t.Logf("Got %d completions:", len(completions))
			for i, c := range completions {
				t.Logf("  %d: '%s' - %s", i, c.Text, c.Description)
			}

			// Check should contain
			for _, expected := range tc.shouldContain {
				found := false
				for _, comp := range completions {
					if comp.Text == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find '%s' in completions", expected)
				}
			}

			// Check should NOT contain
			for _, notExpected := range tc.shouldNOTContain {
				found := false
				for _, comp := range completions {
					if comp.Text == notExpected {
						found = true
						break
					}
				}
				if found {
					t.Errorf("Did not expect to find '%s' in completions", notExpected)
				}
			}
		})
	}
}
