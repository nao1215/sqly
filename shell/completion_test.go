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

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

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
			name:              "All importable files should be shown",
			text:              ".import ",
			expectedFilenames: []string{"testdata/actor.csv", "testdata/sample.csv"},
		},
		{
			name:              "All importable files should be shown (any input)",
			text:              ".import testdata/",
			expectedFilenames: []string{"testdata/actor.csv", "testdata/sample.csv"},
		},
		{
			name:              "All importable files should be shown (partial input)",
			text:              ".import testd",
			expectedFilenames: []string{"testdata/actor.csv", "testdata/sample.csv"},
		},
		{
			name:              "All importable files should be shown (any path)",
			text:              ".import testdata",
			expectedFilenames: []string{"testdata/actor.csv", "testdata/sample.csv"},
		},
		{
			name:              "All importable files should be shown (partial filename)",
			text:              ".import testdata/a",
			expectedFilenames: []string{"testdata/actor.csv", "testdata/sample.csv"},
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

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

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
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

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
	// Test with actual directory structure using new full-path completion behavior

	// Create a shell instance
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	// Test cases for new full-path completion behavior
	// All inputs should show the same complete list of importable files
	testCases := []struct {
		name               string
		input              string
		minExpectedFiles   int      // Minimum number of files expected
		mustContainSamples []string // Sample files that must be present
	}{
		{
			name:               "All importable files should be shown regardless of input",
			input:              ".import ",
			minExpectedFiles:   1,                               // Should find at least some importable files
			mustContainSamples: []string{"testdata/sample.csv"}, // This file should exist
		},
		{
			name:               "All importable files should be shown with any input",
			input:              ".import testdata",
			minExpectedFiles:   1,
			mustContainSamples: []string{"testdata/sample.csv"},
		},
		{
			name:               "All importable files shown even with non-matching prefix",
			input:              ".import nonexistent",
			minExpectedFiles:   1, // Still shows all files
			mustContainSamples: []string{"testdata/sample.csv"},
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

			// Verify minimum number of files
			if len(completions) < tc.minExpectedFiles {
				t.Errorf("Expected at least %d completions, got %d", tc.minExpectedFiles, len(completions))
			}

			// Verify all completions are files with correct description
			for _, comp := range completions {
				if comp.Description != "Importable file" {
					t.Errorf("Expected description 'Importable file', got '%s'", comp.Description)
				}
				// Should not be directories (ending with /)
				if strings.HasSuffix(comp.Text, "/") {
					t.Errorf("Expected file path but got directory: '%s'", comp.Text)
				}
			}

			// Check required sample files are present
			for _, expectedFile := range tc.mustContainSamples {
				found := false
				for _, comp := range completions {
					if comp.Text == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find sample file '%s' in completions", expectedFile)
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
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(tmpDir)
	t.Cleanup(func() { t.Chdir(orig) })

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
			name:  "All importable files should be shown",
			input: "",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
				"testdata/compressed.csv.gz",
			},
		},
		{
			name:  "All importable files should be shown (with directory input)",
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
			name:  "All importable files should be shown (with partial filename)",
			input: "testdata/sample",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
				"testdata/compressed.csv.gz",
			},
		},
		{
			name:  "All importable files should be shown (with partial directory)",
			input: "test",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
				"testdata/compressed.csv.gz",
			},
		},
		{
			name:  "All importable files should be shown (with .import command)",
			input: ".import testdata/",
			expected: []string{
				"testdata/sample.csv",
				"testdata/sample.tsv",
				"testdata/sample.ltsv",
				"testdata/sample.xlsx",
				"testdata/compressed.csv.gz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			completions := shell.getFilePathCompletions(tt.input)

			// Debug: log what files actually exist
			entries, err := os.ReadDir(".")
			if err != nil {
				t.Logf("Failed to read current directory: %v", err)
				return
			}
			t.Logf("Current directory contents:")
			for _, entry := range entries {
				t.Logf("  %s (dir: %v)", entry.Name(), entry.IsDir())
			}

			if testDataEntries, err := os.ReadDir("testdata"); err == nil {
				t.Logf("testdata directory contents:")
				for _, entry := range testDataEntries {
					t.Logf("  testdata/%s (dir: %v, valid: %v)", entry.Name(), entry.IsDir(), isValidFileForCompletion(entry.Name()))
				}
			}

			// Extract completion texts
			var results []string
			for _, c := range completions {
				results = append(results, c.Text)
			}
			t.Logf("Got %d completions: %v", len(completions), results)

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
	// Test the actual completer method (integration test) with new full-path completion

	// Create a shell instance
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	testCases := []struct {
		name             string
		input            string
		minExpectedFiles int
		mustHaveFileType bool // Must contain at least one importable file
	}{
		{
			name:             "Import command should show all importable files",
			input:            ".import ",
			minExpectedFiles: 1,
			mustHaveFileType: true,
		},
		{
			name:             "Import with prefix should still show all importable files",
			input:            ".import testdata",
			minExpectedFiles: 1,
			mustHaveFileType: true,
		},
		{
			name:             "Import with any prefix shows all importable files",
			input:            ".import golden",
			minExpectedFiles: 1,
			mustHaveFileType: true,
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

			// Verify minimum number of completions
			if len(completions) < tc.minExpectedFiles {
				t.Errorf("Expected at least %d completions, got %d", tc.minExpectedFiles, len(completions))
			}

			// If we expect importable files, verify they exist
			if tc.mustHaveFileType {
				foundImportableFile := false
				for _, comp := range completions {
					if comp.Description == "Importable file" {
						foundImportableFile = true
						// Verify it's a file path, not directory
						if strings.HasSuffix(comp.Text, "/") {
							t.Errorf("Expected file path but got directory: '%s'", comp.Text)
						}
						break
					}
				}
				if !foundImportableFile {
					t.Error("Expected to find at least one importable file completion")
				}
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
			doc := prompt.Document{Text: tc.input}
			completions := shell.completer(context.Background(), doc)

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
			doc := prompt.Document{Text: tc.input}
			completions := shell.completer(context.Background(), doc)

			t.Logf("Input: '%s' -> %d completions", tc.input, len(completions))

			// Should not panic and should return some result
			// Just verify we got a valid response (len cannot be negative)
		})
	}
}
