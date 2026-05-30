package shell

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

func newBoundaryTestShell(t *testing.T, usecases Usecases) *Shell {
	t.Helper()

	arg := &config.Arg{
		Output: &config.Output{
			Mode: model.PrintModeTable,
		},
	}
	state, err := newState(arg)
	if err != nil {
		t.Fatalf("newState: %v", err)
	}
	return &Shell{
		argument: arg,
		commands: NewCommands(),
		state:    state,
		usecases: usecases,
	}
}

func hasSuggestionText(suggestions []Suggest, text string) bool {
	for _, suggestion := range suggestions {
		if suggestion.Text == text {
			return true
		}
	}
	return false
}

func TestShell_importDirectory_dependsOnImportUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	dir := t.TempDir()

	before := []*model.Table{
		model.NewTable("users", nil, nil),
	}
	after := []*model.Table{
		model.NewTable("users", nil, nil),
		model.NewTable("orders", nil, nil),
	}

	gomock.InOrder(
		importer.EXPECT().GetTableNames(gomock.Any()).Return(before, nil),
		importer.EXPECT().LoadFiles(gomock.Any(), dir).Return(nil),
		importer.EXPECT().GetTableNames(gomock.Any()).Return(after, nil),
		importer.EXPECT().GetTableNames(gomock.Any()).Return(after, nil),
	)

	s := newBoundaryTestShell(t, Usecases{importer: importer})

	var (
		imported bool
		err      error
	)
	out := captureStdout(t, func() {
		imported, err = s.importDirectory(context.Background(), dir, "fixtures", "")
	})
	if err != nil {
		t.Fatalf("importDirectory returned error: %v", err)
	}
	if !imported {
		t.Fatal("importDirectory reported imported=false, want true")
	}
	if !strings.Contains(out, "Successfully imported 1 table(s) from directory fixtures") {
		t.Fatalf("output %q does not report a successful import", out)
	}
	if !strings.Contains(out, "orders") {
		t.Fatalf("output %q does not mention imported table name", out)
	}
}

func TestShell_importFile_excelSheetFiltering_dependsOnImportAndQueryUsecases(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)
	query := mock.NewMockQueryUsecase(ctrl)
	filePath := "report.xlsx"

	gomock.InOrder(
		importer.EXPECT().IsSupportedFile(filePath).Return(true),
		importer.EXPECT().LoadFiles(gomock.Any(), filePath).Return(nil),
		importer.EXPECT().IsExcelFile(filePath).Return(true),
		importer.EXPECT().GetTableNameFromFilePath(filePath).Return("report"),
		importer.EXPECT().GetTableNames(gomock.Any()).Return([]*model.Table{
			model.NewTable("report_Summary", nil, nil),
			model.NewTable("report_Details", nil, nil),
		}, nil),
		importer.EXPECT().SanitizeForSQL("Summary").Return("Summary"),
		importer.EXPECT().QuoteIdentifier("report_Details").Return(`"report_Details"`),
		query.EXPECT().Exec(gomock.Any(), `DROP TABLE IF EXISTS "report_Details"`).Return(int64(0), nil),
	)

	s := newBoundaryTestShell(t, Usecases{
		importer: importer,
		query:    query,
	})

	if err := s.importFile(context.Background(), filePath, filePath, "Summary"); err != nil {
		t.Fatalf("importFile returned error: %v", err)
	}
}

func TestCommandList_dumpCommand_dependsOnMetadataAndExportUsecases(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	exporter := mock.NewMockExportUsecase(ctrl)

	outputPath := filepath.Join(t.TempDir(), "report.out")
	normalizedPath := normalizeDumpExt(outputPath, model.ExportCSV)
	table := model.NewTable("users", model.NewHeader([]string{"id", "name"}), nil)

	metadata.EXPECT().List(gomock.Any(), "users").Return(table, nil)
	exporter.EXPECT().DumpTable(normalizedPath, table, model.ExportCSV).Return(nil)

	s := newBoundaryTestShell(t, Usecases{
		metadata: metadata,
		export:   exporter,
	})

	out := captureStdout(t, func() {
		if err := NewCommands().dumpCommand(context.Background(), s, []string{"users", outputPath}); err != nil {
			t.Fatalf("dumpCommand returned error: %v", err)
		}
	})
	if !strings.Contains(out, "dump `") || !strings.Contains(out, "table to") {
		t.Fatalf("output %q does not describe dump execution", out)
	}
	if !strings.Contains(out, normalizedPath) {
		t.Fatalf("output %q does not include normalized csv path %q", out, normalizedPath)
	}
}

func TestShell_getRegularCompletions_dependsOnMetadataUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)

	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{
		model.NewTable("users", nil, nil),
	}, nil)
	metadata.EXPECT().Header(gomock.Any(), "users").Return(
		model.NewTable("users", model.NewHeader([]string{"id", "name"}), nil), nil)

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})

	completions := s.getRegularCompletions(context.Background(), "")
	if !hasSuggestionText(completions, "users") {
		t.Fatalf("completions do not include table suggestion: %#v", completions)
	}
	if !hasSuggestionText(completions, "name") {
		t.Fatalf("completions do not include header suggestion: %#v", completions)
	}
}

func TestShell_getFilePathCompletions_dependsOnImportUsecase(t *testing.T) {
	ctrl := gomock.NewController(t)
	importer := mock.NewMockImportUsecase(ctrl)

	tempDir := t.TempDir()
	t.Chdir(tempDir)

	for _, file := range []string{"data.csv", "notes.txt", ".hidden.csv"} {
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", file, err)
		}
	}

	importer.EXPECT().IsSupportedFile("data.csv").Return(true)
	importer.EXPECT().IsSupportedFile("notes.txt").Return(false)

	s := newBoundaryTestShell(t, Usecases{importer: importer})

	completions := s.getFilePathCompletions("")
	if len(completions) != 1 {
		t.Fatalf("expected 1 completion, got %d: %#v", len(completions), completions)
	}
	if completions[0].Text != "data.csv" {
		t.Fatalf("completion text = %q, want %q", completions[0].Text, "data.csv")
	}
	if completions[0].Description != msgImportableFile {
		t.Fatalf("completion description = %q, want %q", completions[0].Description, msgImportableFile)
	}
}
