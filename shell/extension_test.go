package shell

import (
	"runtime"
	"testing"
)

func Test_Ext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{"get extension", "/test/path/to/sample.txt", ".txt"},
		{"hidden file with extension", "/test/path/to/.sample.txt", ".txt"},
		{"no extension", "/test/path/to/sample", ""},
		{"hidden file without extension", "/test/path/to/.sample", ""},
		{"hidden file at current directory", ".sample", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ext(tt.path); got != tt.want {
				t.Errorf("ext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetFileTypeFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		expected string
	}{
		{"CSV file", "test.csv", ".csv"},
		{"TSV file", "test.tsv", ".tsv"},
		{"LTSV file", "test.ltsv", ".ltsv"},
		{"XLSX file", "test.xlsx", ".xlsx"},
		{"JSON file", "test.json", ".json"},
		{"JSONL file", "test.jsonl", ".jsonl"},
		{"Parquet file", "test.parquet", ".parquet"},
		{"Compressed CSV with .gz", "test.csv.gz", ".csv"},
		{"Compressed TSV with .bz2", "test.tsv.bz2", ".tsv"},
		{"Compressed LTSV with .xz", "test.ltsv.xz", ".ltsv"},
		{"Compressed XLSX with .zst", "test.xlsx.zst", ".xlsx"},
		{"Compressed with .snappy", "test.csv.snappy", ".csv"},
		{"Compressed with .s2", "test.csv.s2", ".csv"},
		{"Compressed with .lz4", "test.csv.lz4", ".csv"},
		{"Compressed with .z", "test.csv.z", ".csv"},
		{"Multiple compression extensions", "test.csv.gz.bz2", ".csv"},
		{"No extension", "test", ""},
		{"Only compression extension", "test.gz", ""},
		{"Path with directory", "/path/to/test.csv.gz", ".csv"},
		{"Unsupported file", "test.txt.gz", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getFileTypeFromPath(tt.filePath)
			if result != tt.expected {
				t.Errorf("getFileTypeFromPath(%s) = %s, expected %s", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestExtractSheetNameFromArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		argv     []string
		expected string
	}{
		{"Sheet name found", []string{"file.xlsx", "--sheet=Sheet1"}, "Sheet1"},
		{"Sheet name with spaces", []string{"file.xlsx", "--sheet=My Sheet"}, "My Sheet"},
		{"Multiple arguments with sheet", []string{"file1.xlsx", "--sheet=Data", "file2.csv"}, "Data"},
		{"No sheet argument", []string{"file.xlsx"}, ""},
		{"Empty sheet name", []string{"file.xlsx", "--sheet="}, ""},
		{"First sheet argument wins", []string{"--sheet=First", "--sheet=Second"}, "First"},
		{"Empty arguments", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractSheetNameFromArgs(tt.argv)
			if result != tt.expected {
				t.Errorf("extractSheetNameFromArgs(%v) = %s, expected %s", tt.argv, result, tt.expected)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		shouldError bool
		// unixOnly marks a case that depends on the Unix system-directory block,
		// which keys on Unix absolute paths (e.g. "/etc"). On Windows filepath.Abs
		// rewrites such a path (e.g. "C:\\etc\\passwd"), so the block does not apply
		// and the case is skipped. Ref #427, #428.
		unixOnly bool
	}{
		{name: "Normal file path", path: "test.csv", shouldError: false},
		{name: "Absolute path", path: "/tmp/test.csv", shouldError: false},
		{name: "Single parent directory", path: "../test.csv", shouldError: false},
		{name: "Two parent directories", path: "../../test.csv", shouldError: false},
		{name: "Dangerous path traversal", path: "../../../etc/passwd", shouldError: true},
		{name: "Clean path functionality", path: "./test/../test.csv", shouldError: false},
		// /dev/shm and /dev/fd hold legitimate user inputs and are accepted, while
		// other system directories stay blocked. Ref #427, #428.
		{name: "dev shm user file is allowed", path: "/dev/shm/sqly/user.csv", shouldError: false},
		{name: "dev fd descriptor is allowed", path: "/dev/fd/63", shouldError: false},
		{name: "dev block device is blocked", path: "/dev/sda", shouldError: true, unixOnly: true},
		{name: "etc is blocked", path: "/etc/passwd", shouldError: true, unixOnly: true},
		{name: "proc is blocked", path: "/proc/cpuinfo", shouldError: true, unixOnly: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.unixOnly && runtime.GOOS == "windows" {
				t.Skip("system-directory block applies to Unix absolute paths only")
			}
			_, err := validatePath(tt.path)
			if tt.shouldError && err == nil {
				t.Errorf("validatePath(%s) expected error but got none", tt.path)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("validatePath(%s) unexpected error = %v", tt.path, err)
			}
		})
	}
}
