package shell

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

// TestSessionSettings_AnswerTheSameWayWithNoArgument pins the shape the three
// session-setting commands share. They used to disagree: .dialect reported and
// succeeded, .mode and .row-mismatch failed the script, and .dialect reported on
// stdout where the other two wrote to stderr.
func TestSessionSettings_AnswerTheSameWayWithNoArgument(t *testing.T) {
	// "current <setting>: <value> (available: <values>)", one line, on stderr.
	shape := regexp.MustCompile(`^current [a-z-]+( [a-z-]+)*: \S+ \(available: .+\)\n$`)

	tests := []struct {
		name    string
		run     func(t *testing.T, s *Shell) error
		setting string
		want    string
	}{
		{
			name:    ".dialect",
			run:     func(_ *testing.T, s *Shell) error { return NewCommands().dialectCommand(context.Background(), s, nil) },
			setting: settingDialect,
			want:    "sqlite",
		},
		{
			name:    ".mode",
			run:     func(_ *testing.T, s *Shell) error { return NewCommands().modeCommand(context.Background(), s, nil) },
			setting: settingOutputMode,
			want:    "table",
		},
		{
			name: ".row-mismatch",
			run: func(_ *testing.T, s *Shell) error {
				return NewCommands().rowMismatchCommand(context.Background(), s, nil)
			},
			setting: settingRowMismatch,
			want:    "error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := mock.NewMockQueryUsecase(ctrl)
			query.EXPECT().Dialect().Return(dialect.SQLite).AnyTimes()
			s := newBoundaryTestShell(t, Usecases{query: query})

			var stderr string
			stdout := captureStdout(t, func() {
				stderr = captureStderr(t, func() {
					if err := tt.run(t, s); err != nil {
						t.Fatalf("%s with no argument = %v, want the current value", tt.name, err)
					}
				})
			})

			if stdout != "" {
				t.Errorf("%s wrote %q to stdout; stdout carries data only", tt.name, stdout)
			}
			if !shape.MatchString(stderr) {
				t.Errorf("%s reported %q, want one line of the shared shape", tt.name, stderr)
			}
			if !strings.HasPrefix(stderr, "current "+tt.setting+": "+tt.want+" ") {
				t.Errorf("%s reported %q, want the current %s (%s)", tt.name, stderr, tt.setting, tt.want)
			}
		})
	}
}
