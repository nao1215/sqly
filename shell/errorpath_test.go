package shell

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

// covErrTable is a minimal non-empty table used where a usecase must return a
// value so a later call is the one that fails.
func covErrTable() *model.Table {
	return model.NewTable("t", model.NewHeader([]string{"n"}), []model.Record{
		model.NewRecord([]string{"1"}),
	})
}

// covErrEmptyTable is a zero-row, zero-column table, which the helper commands
// treat as "table does not exist".
func covErrEmptyTable() *model.Table {
	return model.NewTable("t", model.Header{}, []model.Record{})
}

func TestCommandList_headerCommand_propagatesUsecaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().Header(gomock.Any(), "user").Return(nil, errors.New("no such table: user"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := s.commands.headerCommand(context.Background(), s, []string{"user"})
	if err == nil {
		t.Fatal("want error when metadata.Header fails, got nil")
	}
}

func TestCommandList_tablesCommand_propagatesUsecaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().SchemaObjects(gomock.Any()).Return(nil, errors.New("db closed"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := s.commands.tablesCommand(context.Background(), s, []string{})
	if err == nil {
		t.Fatal("want error when metadata.SchemaObjects fails, got nil")
	}
}

func TestCommandList_describeCommand_propagatesQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().QuoteIdentifier(gomock.Any()).DoAndReturn(func(s string) string { return s }).AnyTimes()
	// Both objectExists and tableColumns issue a Query; failing every Query drives
	// the error-return branch in tableColumns and describeCommand.
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, errors.New("query failed")).AnyTimes()

	s := newBoundaryTestShell(t, Usecases{query: query, importer: importer})
	err := s.commands.describeCommand(context.Background(), s, []string{"user"})
	if err == nil {
		t.Fatal("want error when the column query fails, got nil")
	}
}

func TestCommandList_describeCommand_reportsMissingTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().QuoteIdentifier(gomock.Any()).DoAndReturn(func(s string) string { return s }).AnyTimes()
	// A PRAGMA table_info on a missing table returns no rows, which describeCommand
	// turns into a "no such table" error.
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(covErrEmptyTable(), nil).AnyTimes()

	s := newBoundaryTestShell(t, Usecases{query: query, importer: importer})
	err := s.commands.describeCommand(context.Background(), s, []string{"ghost"})
	if err == nil {
		t.Fatal("want no-such-table error, got nil")
	}
}

func TestCommandList_schemaCommand_propagatesQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().QuoteIdentifier(gomock.Any()).DoAndReturn(func(s string) string { return s }).AnyTimes()
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, errors.New("query failed")).AnyTimes()

	s := newBoundaryTestShell(t, Usecases{query: query, importer: importer})
	err := s.commands.schemaCommand(context.Background(), s, []string{"user"})
	if err == nil {
		t.Fatal("want error when the schema lookup fails, got nil")
	}
}

func TestCommandList_schemaCommand_reportsMissingTable(t *testing.T) {
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().QuoteIdentifier(gomock.Any()).DoAndReturn(func(s string) string { return s }).AnyTimes()
	// storedCreateSQL and the tableColumns fallback both see no rows, so the
	// synthesized-path fallback reports the table as missing.
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(covErrEmptyTable(), nil).AnyTimes()

	s := newBoundaryTestShell(t, Usecases{query: query, importer: importer})
	err := s.commands.schemaCommand(context.Background(), s, []string{"ghost"})
	if err == nil {
		t.Fatal("want no-such-table error, got nil")
	}
}

func TestCommandList_dumpCommand_propagatesExportError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	export := mock.NewMockExportUsecase(ctrl)
	metadata.EXPECT().List(gomock.Any(), "t").Return(covErrTable(), nil)
	export.EXPECT().DumpTable(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("disk full"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata, export: export})
	dest := filepath.Join(t.TempDir(), "out.csv")
	err := s.commands.dumpCommand(context.Background(), s, []string{"t", dest})
	if err == nil {
		t.Fatal("want error when export.DumpTable fails, got nil")
	}
}

func TestCommandList_dumpCommand_propagatesListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().List(gomock.Any(), "t").Return(nil, errors.New("no such table"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	dest := filepath.Join(t.TempDir(), "out.csv")
	err := s.commands.dumpCommand(context.Background(), s, []string{"t", dest})
	if err == nil {
		t.Fatal("want error when metadata.List fails, got nil")
	}
}

func TestShell_runInspect_propagatesColumnQueryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{covErrTable()}, nil)
	importer.EXPECT().QuoteIdentifier(gomock.Any()).DoAndReturn(func(s string) string { return s }).AnyTimes()
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, errors.New("query failed")).AnyTimes()

	s := newBoundaryTestShell(t, Usecases{metadata: metadata, query: query, importer: importer})
	err := s.runInspect(context.Background())
	if err == nil {
		t.Fatal("want error when inspecting a table whose column query fails, got nil")
	}
}

func TestShell_runInspect_reportsNoTables(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{}, nil)

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := s.runInspect(context.Background())
	if err == nil {
		t.Fatal("want error when there are no tables to inspect, got nil")
	}
}

// TestShell_negativeInspectSample_isRejectedBeforeTheRun records where the
// negative-count refusal lives now: in argument parsing, so the run exits 2 as a
// usage error having read nothing, rather than exiting 1 from runInspect after
// the import had already happened. runInspect itself can no longer see a
// negative value, which is why it no longer checks for one.
func TestShell_negativeInspectSample_isRejectedBeforeTheRun(t *testing.T) {
	t.Parallel()

	arg, err := config.NewArg([]string{"sqly", "--inspect", "--inspect-sample", "-1", "data.csv"})
	if err == nil {
		t.Fatalf("want a usage error for a negative --inspect-sample, got arg = %+v", arg)
	}
	var argErr *config.ArgError
	if !errors.As(err, &argErr) {
		t.Fatalf("error %v is not a config.ArgError, so it would not exit %d", err, ExitUsage)
	}
	if code := ExitCode(err); code != ExitUsage {
		t.Errorf("exit code = %d, want %d", code, ExitUsage)
	}
}

func TestShellInspectRowCountBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		result   *model.Table
		queryErr error
		wantErr  bool
	}{
		{name: "empty result", result: covErrEmptyTable()},
		{name: "invalid count", result: covErrTableWithValue("not-a-number"), wantErr: true},
		{name: "query error", queryErr: errors.New("query failed"), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := mock.NewMockQueryUsecase(ctrl)
			importer := mock.NewMockImportUsecase(ctrl)
			importer.EXPECT().QuoteIdentifier("t").Return("t")
			query.EXPECT().Query(gomock.Any(), "SELECT COUNT(*) FROM t").Return(test.result, test.queryErr)
			s := newBoundaryTestShell(t, Usecases{query: query, importer: importer})
			count, err := s.inspectRowCount(context.Background(), "t")
			if test.wantErr != (err != nil) {
				t.Fatalf("inspectRowCount() error = %v, wantErr=%v", err, test.wantErr)
			}
			if !test.wantErr && count != 0 {
				t.Errorf("empty result count = %d, want 0", count)
			}
		})
	}
}

func TestShellInspectSampleBoundaries(t *testing.T) {
	s := newBoundaryTestShell(t, Usecases{})
	sample, err := s.inspectSample(context.Background(), "t", 0)
	if err != nil || string(sample) != "[]" {
		t.Fatalf("inspectSample(limit=0) = %s, %v, want []", sample, err)
	}

	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().QuoteIdentifier("t").Return("t").AnyTimes()
	// The sample is ordered by rowid; a table with no rowid falls back to a plain
	// scan, so both spellings are attempted and both fail here.
	query.EXPECT().Query(gomock.Any(), "SELECT * FROM t ORDER BY rowid LIMIT 2").Return(nil, errors.New("sample failed"))
	query.EXPECT().Query(gomock.Any(), "SELECT * FROM t LIMIT 2").Return(nil, errors.New("sample failed"))
	s = newBoundaryTestShell(t, Usecases{query: query, importer: importer})
	if _, err := s.inspectSample(context.Background(), "t", 2); err == nil {
		t.Fatal("inspectSample query failure returned nil error")
	}
}

func covErrTableWithValue(value string) *model.Table {
	return model.NewTable("t", model.NewHeader([]string{"count"}), []model.Record{model.NewRecord([]string{value})})
}
