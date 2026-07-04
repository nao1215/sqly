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

func TestCommandList_importModeCommand_SetsPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	// The command must push the new policy down to the importer so later imports
	// honor it.
	importer.EXPECT().SetMalformedRowPolicy(model.MalformedRowFill).Times(1)

	shell := newBoundaryTestShell(t, Usecases{importer: importer})

	backup := config.Stderr
	defer func() { config.Stderr = backup }()
	var buf bytes.Buffer
	config.Stderr = &buf

	if err := NewCommands().importModeCommand(context.Background(), shell, []string{"fill"}); err != nil {
		t.Fatalf("importModeCommand returned error: %v", err)
	}
	if shell.state.importMode != model.MalformedRowFill {
		t.Fatalf("state.importMode = %v, want fill", shell.state.importMode)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Change import mode from stop to fill")) {
		t.Fatalf("banner = %q, want it to report the change", buf.String())
	}
}

func TestCommandList_importModeCommand_Errors(t *testing.T) {
	tests := []struct {
		name string
		argv []string
	}{
		{name: "no argument reports usage as an error", argv: nil},
		{name: "unknown policy is rejected", argv: []string{"keep"}},
		{name: "more than one argument is rejected", argv: []string{"skip", "fill"}},
		{name: "setting the current policy is rejected", argv: []string{"stop"}},
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

			if err := NewCommands().importModeCommand(context.Background(), shell, tt.argv); err == nil {
				t.Fatalf("expected an error for argv %v, got nil", tt.argv)
			}
			if shell.state.importMode != model.MalformedRowStop {
				t.Fatalf("state.importMode changed to %v on an error path", shell.state.importMode)
			}
		})
	}
}
