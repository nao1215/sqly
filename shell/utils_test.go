package shell

import (
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

func Test_isJSON(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "it's json file",
			args: args{
				path: "/test/path/to/sample.json",
			},
			want: true,
		},
		{
			name: "not json, it's text file",
			args: args{
				path: "/test/path/to/sample.txt",
			},
			want: false,
		},
		{
			name: "it's json file: file is hidden one with extension",
			args: args{
				path: "/test/path/to/.sample.json",
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
			if got := isJSON(tt.args.path); got != tt.want {
				t.Errorf("isJSON() = %v, want %v", got, tt.want)
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
