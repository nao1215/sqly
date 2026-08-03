package shell

import (
	"bytes"
	"context"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func TestCommandList_rowMismatchCommand_SetsPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	// The command must push the new policy down to the importer so later imports
	// honor it.
	importer.EXPECT().SetRowMismatchPolicy(model.RowMismatchPad).Times(1)

	shell := newBoundaryTestShell(t, Usecases{importer: importer})

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var buf bytes.Buffer
	config.Stderr = &buf

	if err := NewCommands().rowMismatchCommand(context.Background(), shell, []string{"pad"}); err != nil {
		t.Fatalf("rowMismatchCommand returned error: %v", err)
	}
	if shell.state.rowMismatch != model.RowMismatchPad {
		t.Fatalf("state.rowMismatch = %v, want pad", shell.state.rowMismatch)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Change row-mismatch policy from error to pad")) {
		t.Fatalf("banner = %q, want it to report the change", buf.String())
	}
}

func TestCommandList_rowMismatchCommand_Errors(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{name: "no argument reports usage as an error", argv: nil},
		{name: "unknown policy is rejected", argv: []string{"keep"}},
		{name: "more than one argument is rejected", argv: []string{"skip", "pad"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			importer := mock.NewMockImportUsecase(ctrl)
			// No policy change is expected on any error path.
			shell := newBoundaryTestShell(t, Usecases{importer: importer})

			backup := config.Stderr
			defer func() { config.Stderr = backup }()
			var buf bytes.Buffer
			config.Stderr = &buf

			if err := NewCommands().rowMismatchCommand(context.Background(), shell, tt.argv); err == nil {
				t.Fatalf("expected an error for argv %v, got nil", tt.argv)
			}
			if shell.state.rowMismatch != model.RowMismatchError {
				t.Fatalf("state.rowMismatch changed to %v on an error path", shell.state.rowMismatch)
			}
		})
	}
}

// TestCommandList_rowMismatchCommand_CurrentPolicyIsANoOp pins that selecting the
// policy already in effect succeeds quietly. It used to be an error, which made
// a batch script fatal on a line that changed nothing — including the natural
// combination of --row-mismatch with a script that restates the same policy.
func TestCommandList_rowMismatchCommand_CurrentPolicyIsANoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	// No SetRowMismatchPolicy call is expected: nothing changed.
	shell := newBoundaryTestShell(t, Usecases{importer: importer})

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var buf bytes.Buffer
	config.Stderr = &buf

	if err := NewCommands().rowMismatchCommand(context.Background(), shell, []string{"error"}); err != nil {
		t.Fatalf("selecting the current policy returned %v, want nil", err)
	}
	if shell.state.rowMismatch != model.RowMismatchError {
		t.Fatalf("state.rowMismatch = %v, want it unchanged", shell.state.rowMismatch)
	}
	if buf.Len() != 0 {
		t.Errorf("a no-op wrote %q to stderr, want nothing", buf.String())
	}
}
