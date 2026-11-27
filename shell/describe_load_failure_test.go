package shell

import (
	"errors"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestDescribeLoadFailureNamesTheInputTheUserTyped checks which input a failed
// load is blamed on, and that the message quotes what the user named rather
// than a staged copy: the loader's own report when it names a path, else the
// longest input path the message contains.
func TestDescribeLoadFailureNamesTheInputTheUserTyped(t *testing.T) {
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	plan := &importPlan{targets: []importTarget{
		{loadPath: "/data/x", displayPath: "/data/x"},
		{loadPath: "/data/xy", displayPath: "/data/xy"},
		{loadPath: "/tmp/stage/remote.csv", displayPath: "https://example.com/remote.csv"},
	}}
	cause := errors.New("bad row")

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "the loader names the second input",
			err:  &model.ImportError{Path: "/data/xy", Err: cause},
			want: "failed to import file /data/xy: bad row",
		},
		{
			name: "the loader names the first input",
			err:  &model.ImportError{Path: "/data/x", Err: cause},
			want: "failed to import file /data/x: bad row",
		},
		{
			name: "a message holding two paths, one inside the other",
			err:  errors.New("open /data/xy: denied"),
			want: "failed to import file /data/xy: open /data/xy: denied",
		},
		{
			name: "a message quoting a staged copy",
			err:  errors.New("parse /tmp/stage/remote.csv: bad row"),
			want: "failed to import file https://example.com/remote.csv: parse https://example.com/remote.csv: bad row",
		},
		{
			name: "the loader names a path that is no input",
			err:  &model.ImportError{Path: "/elsewhere", Err: cause},
			want: "/elsewhere: bad row",
		},
		{
			name: "a message naming no input",
			err:  cause,
			want: "bad row",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.describeLoadFailure(plan, tt.err)
			if got.Error() != tt.want {
				t.Errorf("describeLoadFailure() = %q, want %q", got, tt.want)
			}
			// A cause the caller tests for survives where it was wrapped.
			if errors.Is(tt.err, cause) && !errors.Is(got, cause) {
				t.Errorf("describeLoadFailure() lost the cause %v", cause)
			}
		})
	}
}
