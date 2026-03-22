package shell

import (
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
	}{
		{"Normal file path", "test.csv", false},
		{"Absolute path", "/tmp/test.csv", false},
		{"Single parent directory", "../test.csv", false},
		{"Two parent directories", "../../test.csv", false},
		{"Dangerous path traversal", "../../../etc/passwd", true},
		{"Clean path functionality", "./test/../test.csv", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
