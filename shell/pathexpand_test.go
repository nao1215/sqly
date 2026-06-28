package shell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTildeWithHome(t *testing.T) {
	t.Parallel()

	home := filepath.Join(string(filepath.Separator)+"home", "nao")

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "bare tilde maps to home",
			path: "~",
			want: home,
		},
		{
			name: "tilde slash subdir joins home",
			path: "~/project/data.csv",
			want: filepath.Join(home, "project", "data.csv"),
		},
		{
			name: "tilde user form is left unchanged",
			path: "~other/data.csv",
			want: "~other/data.csv",
		},
		{
			name: "tilde later in the path is left unchanged",
			path: "data/~backup/file.csv",
			want: "data/~backup/file.csv",
		},
		{
			name: "plain relative path is left unchanged",
			path: "testdata/user.csv",
			want: "testdata/user.csv",
		},
		{
			name: "absolute path is left unchanged",
			path: filepath.Join(home, "file.csv"),
			want: filepath.Join(home, "file.csv"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := expandTildeWithHome(tt.path, home); got != tt.want {
				t.Errorf("expandTildeWithHome(%q, %q) = %q, want %q", tt.path, home, got, tt.want)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home directory: %v", err)
	}

	t.Run("bare tilde resolves to the real home directory", func(t *testing.T) {
		t.Parallel()

		got, err := expandTilde("~")
		if err != nil {
			t.Fatalf("expandTilde returned error: %v", err)
		}
		if got != home {
			t.Errorf("expandTilde(\"~\") = %q, want %q", got, home)
		}
	})

	t.Run("non-tilde path is returned unchanged", func(t *testing.T) {
		t.Parallel()

		got, err := expandTilde("testdata/user.csv")
		if err != nil {
			t.Fatalf("expandTilde returned error: %v", err)
		}
		if got != "testdata/user.csv" {
			t.Errorf("expandTilde = %q, want unchanged", got)
		}
	})
}
