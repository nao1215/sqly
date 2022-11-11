package shell

import (
	"testing"
)

func Test_ext(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ext(tt.args.path); got != tt.want {
				t.Errorf("ext() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
