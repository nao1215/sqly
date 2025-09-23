package shell

import (
	"runtime"
	"strings"
	"testing"
)

func Test_isCSV(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's csv file",
			args: args{
				path: "/test/path/to/sample.csv",
			},
			want: true,
		},
		{
			name: "not csv, it's text file",
			args: args{
				path: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's csv file: file is hidden one with extension",
			args: args{
				path: "/test/path/to/.sample.csv",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				path: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				path: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCSV(tt.args.path); got != tt.want {
				t.Errorf("isCSV() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isTSV(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's tsv file",
			args: args{
				filePath: "/test/path/to/sample.tsv",
			},
			want: true,
		},
		{
			name: "not tsv, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's tsv file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.tsv",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTSV(tt.args.filePath); got != tt.want {
				t.Errorf("isTSV() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isLTSV(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's ltsv file",
			args: args{
				filePath: "/test/path/to/sample.ltsv",
			},
			want: true,
		},
		{
			name: "not ltsv, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's ltsv file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.ltsv",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isLTSV(tt.args.filePath); got != tt.want {
				t.Errorf("isLTSV() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isXLAM(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's xlam file",
			args: args{
				filePath: "/test/path/to/sample.xlam",
			},
			want: true,
		},
		{
			name: "not xlam, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's xlam file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.xlam",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXLAM(tt.args.filePath); got != tt.want {
				t.Errorf("isXLAM() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isXLSM(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's xlsm file",
			args: args{
				filePath: "/test/path/to/sample.xlsm",
			},
			want: true,
		},
		{
			name: "not xlsm, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's xlsm file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.xlsm",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXLSM(tt.args.filePath); got != tt.want {
				t.Errorf("isXLSM() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isXLSX(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's xlsx file",
			args: args{
				filePath: "/test/path/to/sample.xlsx",
			},
			want: true,
		},
		{
			name: "not xlsx, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's xlsx file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.xlsx",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXLSX(tt.args.filePath); got != tt.want {
				t.Errorf("isXLSX() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isXLTM(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's xltm file",
			args: args{
				filePath: "/test/path/to/sample.xltm",
			},
			want: true,
		},
		{
			name: "not xltm, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's xltm file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.xltm",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXLTM(tt.args.filePath); got != tt.want {
				t.Errorf("isXLTM() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isXLTX(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's xltx file",
			args: args{
				filePath: "/test/path/to/sample.xltx",
			},
			want: true,
		},
		{
			name: "not xltx, it's text file",
			args: args{
				filePath: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's xltx file: file is hidden one with extension",
			args: args{
				filePath: "/test/path/to/.sample.xltx",
			},
			want: true,
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				filePath: "/test/path/to/sample",
			},
			want: false,
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				filePath: "/test/path/to/.sample",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isXLTX(tt.args.filePath); got != tt.want {
				t.Errorf("isXLTX() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Ext(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "get extension",
			args: args{
				path: "/test/path/to/sample.txt",
			},
			want: ".txt",
		},
		{
			name: "get extension: file is hidden one with extension",
			args: args{
				path: "/test/path/to/.sample.txt",
			},
			want: ".txt",
		},
		{
			name: "not get extension: no extension in path",
			args: args{
				path: "/test/path/to/sample",
			},
			want: "",
		},
		{
			name: "not get extension: file is hidden one without extension",
			args: args{
				path: "/test/path/to/.sample",
			},
			want: "",
		},
		{
			name: "not get extension: hidden file at current directory",
			args: args{
				path: ".sample",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ext(tt.args.path); got != tt.want {
				t.Errorf("Ext() = %v, want %v", got, tt.want)
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
		{"Compressed CSV with .gz", "test.csv.gz", ".csv"},
		{"Compressed TSV with .bz2", "test.tsv.bz2", ".tsv"},
		{"Compressed LTSV with .xz", "test.ltsv.xz", ".ltsv"},
		{"Compressed XLSX with .zst", "test.xlsx.zst", ".xlsx"},
		{"Multiple compression extensions", "test.csv.gz.bz2", ".csv"},
		{"No extension", "test", ""},
		{"Only compression extension", "test.gz", ""},
		{"Path with directory", "/path/to/test.csv.gz", ".csv"},
		{"Unsupported file", "test.txt.gz", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFileTypeFromPath(tt.filePath)
			if result != tt.expected {
				t.Errorf("getFileTypeFromPath(%s) = %s, expected %s", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestCompressedFileDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filePath string
		isCSV    bool
		isXLSX   bool
	}{
		{"Regular CSV", "test.csv", true, false},
		{"Regular XLSX", "test.xlsx", false, true},
		{"Compressed CSV (.gz)", "test.csv.gz", true, false},
		{"Compressed CSV (.bz2)", "test.csv.bz2", true, false},
		{"Compressed CSV (.xz)", "test.csv.xz", true, false},
		{"Compressed CSV (.zst)", "test.csv.zst", true, false},
		{"Compressed XLSX (.gz)", "test.xlsx.gz", false, true},
		{"Compressed XLSX (.bz2)", "test.xlsx.bz2", false, true},
		{"Compressed XLSX (.xz)", "test.xlsx.xz", false, true},
		{"Compressed XLSX (.zst)", "test.xlsx.zst", false, true},
		{"Multiple compression", "test.csv.gz.bz2", true, false},
		{"Unsupported compressed", "test.txt.gz", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvResult := isCSV(tt.filePath)
			if csvResult != tt.isCSV {
				t.Errorf("isCSV(%s) = %v, expected %v", tt.filePath, csvResult, tt.isCSV)
			}

			xlsxResult := isXLSX(tt.filePath)
			if xlsxResult != tt.isXLSX {
				t.Errorf("isXLSX(%s) = %v, expected %v", tt.filePath, xlsxResult, tt.isXLSX)
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
		{
			name:     "Sheet name found",
			argv:     []string{"file.xlsx", "--sheet=Sheet1"},
			expected: "Sheet1",
		},
		{
			name:     "Sheet name with spaces",
			argv:     []string{"file.xlsx", "--sheet=My Sheet"},
			expected: "My Sheet",
		},
		{
			name:     "Multiple arguments with sheet",
			argv:     []string{"file1.xlsx", "--sheet=Data", "file2.csv"},
			expected: "Data",
		},
		{
			name:     "No sheet argument",
			argv:     []string{"file.xlsx"},
			expected: "",
		},
		{
			name:     "Empty sheet name",
			argv:     []string{"file.xlsx", "--sheet="},
			expected: "",
		},
		{
			name:     "Sheet argument in middle",
			argv:     []string{"file1.xlsx", "file2.csv", "--sheet=Summary", "file3.tsv"},
			expected: "Summary",
		},
		{
			name:     "First sheet argument wins",
			argv:     []string{"--sheet=First", "--sheet=Second"},
			expected: "First",
		},
		{
			name:     "Empty arguments",
			argv:     []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
		errorMsg    string
	}{
		{
			name:        "Normal file path",
			path:        "test.csv",
			shouldError: false,
		},
		{
			name:        "Absolute path",
			path:        "/tmp/test.csv",
			shouldError: false,
		},
		{
			name:        "Path with single parent directory",
			path:        "../test.csv",
			shouldError: false,
		},
		{
			name:        "Path with two parent directories",
			path:        "../../test.csv",
			shouldError: false,
		},
		{
			name:        "Dangerous path traversal - triple parent",
			path:        "../../../etc/passwd",
			shouldError: true,
			errorMsg:    "potentially dangerous path pattern detected",
		},
		{
			name:        "Windows path traversal",
			path:        "..\\..\\..\\windows\\system32",
			shouldError: true,
			errorMsg:    "potentially dangerous path pattern detected",
		},
		{
			name:        "Double encoding attempt",
			path:        "....//etc/passwd",
			shouldError: true,
			errorMsg:    "potentially dangerous path pattern detected",
		},
		{
			name:        "URL encoded path traversal",
			path:        "..%2f..%2f..%2fetc/passwd",
			shouldError: true,
			errorMsg:    "potentially dangerous path pattern detected",
		},
		{
			name:        "System directory /etc (Unix only)",
			path:        "/etc/passwd",
			shouldError: true,
			errorMsg:    "access to system directory not allowed",
		},
		{
			name:        "System directory /proc (Unix only)",
			path:        "/proc/version",
			shouldError: true,
			errorMsg:    "access to system directory not allowed",
		},
		{
			name:        "Clean path functionality",
			path:        "./test/../test.csv",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip Unix-specific system directory tests on Windows
			if runtime.GOOS == "windows" && strings.Contains(tt.name, "(Unix only)") {
				t.Skip("Skipping Unix-specific test on Windows")
			}

			cleanPath, err := validatePath(tt.path)

			if tt.shouldError {
				if err == nil {
					t.Errorf("validatePath(%s) expected error but got none", tt.path)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("validatePath(%s) error = %v, expected to contain %s", tt.path, err, tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validatePath(%s) unexpected error = %v", tt.path, err)
					return
				}
				if cleanPath == "" {
					t.Errorf("validatePath(%s) returned empty clean path", tt.path)
				}
			}
		})
	}
}
