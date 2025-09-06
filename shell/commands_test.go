package shell

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

func TestCommandList_cdCommand(t *testing.T) {
	// Note: Cannot use t.Parallel() because of t.Chdir() and t.Setenv() usage
	tests := []struct {
		name      string
		argv      []string
		wantError bool
		setup     func() (string, func()) // Returns initial dir and cleanup function
	}{
		{
			name: "change to specified directory",
			argv: []string{"."},
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
		{
			name: "change to home directory when no args",
			argv: []string{},
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				originalHome := os.Getenv("HOME")
				// Set a test HOME directory (use existing directory to avoid Windows cleanup issues)
				t.Setenv("HOME", cwd)
				return cwd, func() {
					t.Chdir(cwd)
					t.Setenv("HOME", originalHome)
				}
			},
		},
		{
			name:      "error with too many arguments",
			argv:      []string{"dir1", "dir2"},
			wantError: true,
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
		{
			name:      "error with non-existent directory",
			argv:      []string{"/nonexistent/directory"},
			wantError: true,
			setup: func() (string, func()) {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				return cwd, func() {
					t.Chdir(cwd)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure we return to the original directory after each test
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("Could not get working directory: %v", err)
			}
			defer func() {
				t.Chdir(originalWd)
			}()

			// Setup
			originalDir, cleanup := tt.setup()
			defer cleanup()

			// Create shell instance
			arg := &config.Arg{
				Output: &config.Output{
					Mode: model.PrintModeTable,
				},
			}
			state, err := newState(arg)
			if err != nil {
				t.Fatalf("Failed to create state: %v", err)
			}
			shell := &Shell{
				argument: arg,
				config:   &config.Config{},
				state:    state,
			}
			shell.state.cwd = originalDir

			// Create command list
			commandList := CommandList{}

			// Execute command
			err = commandList.cdCommand(context.Background(), shell, tt.argv)

			// Check result
			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCommandList_lsCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		argv      []string
		wantError bool
	}{
		{
			name: "list current directory",
			argv: []string{},
		},
		{
			name: "list specified directory",
			argv: []string{"."},
		},
		{
			name:      "error with too many arguments",
			argv:      []string{"dir1", "dir2"},
			wantError: true,
		},
		{
			name:      "error with non-existent directory",
			argv:      []string{"/nonexistent/directory"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create shell instance
			arg := &config.Arg{
				Output: &config.Output{
					Mode: model.PrintModeTable,
				},
			}
			state, err := newState(arg)
			if err != nil {
				t.Fatalf("Failed to create state: %v", err)
			}
			shell := &Shell{
				argument: arg,
				config:   &config.Config{},
				state:    state,
			}

			// Create command list
			commandList := CommandList{}

			// Execute command
			err = commandList.lsCommand(context.Background(), shell, tt.argv)

			// Check result
			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestShell_getFilePathCompletions_errorHandling(t *testing.T) {
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	// Test with a directory that should cause a permission error or similar
	// This is tricky to test reliably across platforms, so we test the fallback behavior
	// Create a temporary directory and use t.Chdir
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Create some test files
	for _, filename := range []string{"test.csv", "test.tsv", "readme.txt"} {
		func() {
			file, err := os.Create(filepath.Clean(filename))
			if err != nil {
				t.Fatalf("Could not create test file %s: %v", filename, err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Could not close test file %s: %v", filename, err)
				}
			}()
		}()
	}

	// Test completion - should work normally
	completions := shell.getFilePathCompletions(".import ")

	if len(completions) == 0 {
		t.Error("Expected some completions in test directory")
	}

	// Verify only valid files are returned
	hasCSV := false
	hasTSV := false
	hasInvalidFile := false

	for _, comp := range completions {
		if comp.Text == "test.csv" {
			hasCSV = true
		}
		if comp.Text == "test.tsv" {
			hasTSV = true
		}
		if comp.Text == "readme.txt" {
			hasInvalidFile = true
		}
		if comp.Description != "Importable file" {
			t.Errorf("Expected description 'Importable file', got '%s'", comp.Description)
		}
	}

	if !hasCSV {
		t.Error("Expected to find test.csv in completions")
	}
	if !hasTSV {
		t.Error("Expected to find test.tsv in completions")
	}
	if hasInvalidFile {
		t.Error("Should not find readme.txt in completions")
	}
}

func TestShell_getFilePathCompletions_edgeCases(t *testing.T) {
	shell := &Shell{
		argument: &config.Arg{},
		config:   &config.Config{},
	}

	// Test with different directory structures
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Create nested directory structure
	nestedDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(nestedDir, 0750)
	if err != nil {
		t.Fatalf("Could not create nested directory: %v", err)
	}

	// Create files in nested directory
	nestedFile := filepath.Join(nestedDir, "nested.csv")
	func() {
		file, err := os.Create(filepath.Clean(nestedFile))
		if err != nil {
			t.Fatalf("Could not create nested file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Could not close nested file: %v", err)
			}
		}()
	}()

	// Create hidden directory (should be skipped)
	hiddenDir := filepath.Join(tempDir, ".hidden")
	err = os.MkdirAll(hiddenDir, 0750)
	if err != nil {
		t.Fatalf("Could not create hidden directory: %v", err)
	}

	hiddenFile := filepath.Join(hiddenDir, "hidden.csv")
	func() {
		file, err := os.Create(filepath.Clean(hiddenFile))
		if err != nil {
			t.Fatalf("Could not create hidden file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Could not close hidden file: %v", err)
			}
		}()
	}()

	// Test completion
	completions := shell.getFilePathCompletions(".import ")

	// Should find nested file but not hidden file
	foundNested := false
	foundHidden := false

	for _, comp := range completions {
		if filepath.ToSlash(comp.Text) == "subdir/nested.csv" {
			foundNested = true
		}
		if filepath.ToSlash(comp.Text) == ".hidden/hidden.csv" {
			foundHidden = true
		}
	}

	if !foundNested {
		t.Error("Expected to find nested.csv in subdir")
	}
	if foundHidden {
		t.Error("Should not find hidden.csv in .hidden directory")
	}
}
