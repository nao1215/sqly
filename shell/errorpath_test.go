package shell

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

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

func TestShell_runInspect_rejectsNegativeSample(t *testing.T) {
	s := newBoundaryTestShell(t, Usecases{})
	s.argument.InspectSample = -1
	err := s.runInspect(context.Background())
	if err == nil {
		t.Fatal("want error for a negative --inspect-sample, got nil")
	}
}

func TestShell_runProfile_propagatesListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return(nil, errors.New("db closed"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := s.runProfile(context.Background())
	if err == nil {
		t.Fatal("want error when metadata.TablesName fails, got nil")
	}
}

func TestShell_runProfile_reportsNoTables(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{}, nil)

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := s.runProfile(context.Background())
	if err == nil {
		t.Fatal("want error when there are no tables to profile, got nil")
	}
}

func TestShell_runCompare_propagatesListError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return(nil, errors.New("db closed"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	if err := s.runCompare(context.Background()); err == nil {
		t.Fatal("want error when metadata.TablesName fails, got nil")
	}
}

func TestShell_resolveCompareTables(t *testing.T) {
	newMetaShell := func(t *testing.T, tables []*model.Table, listErr error) *Shell {
		t.Helper()
		ctrl := gomock.NewController(t)
		metadata := mock.NewMockMetadataUsecase(ctrl)
		metadata.EXPECT().TablesName(gomock.Any()).Return(tables, listErr).AnyTimes()
		return newBoundaryTestShell(t, Usecases{metadata: metadata})
	}

	t.Run("rejects an import that produced no tables", func(t *testing.T) {
		s := newMetaShell(t, []*model.Table{}, nil)
		if _, _, err := s.resolveCompareTables(context.Background()); err == nil {
			t.Fatal("want error for zero tables, got nil")
		}
	})

	t.Run("rejects an import that produced more than two tables", func(t *testing.T) {
		three := []*model.Table{
			model.NewTable("a", model.Header{}, nil),
			model.NewTable("b", model.Header{}, nil),
			model.NewTable("c", model.Header{}, nil),
		}
		s := newMetaShell(t, three, nil)
		if _, _, err := s.resolveCompareTables(context.Background()); err == nil {
			t.Fatal("want error for three tables, got nil")
		}
	})

	t.Run("rejects a --compare-tables spec that does not name exactly two tables", func(t *testing.T) {
		s := newBoundaryTestShell(t, Usecases{})
		s.argument.CompareTables = "only-one"
		if _, _, err := s.resolveCompareTables(context.Background()); err == nil {
			t.Fatal("want error for a one-name spec, got nil")
		}
	})

	t.Run("rejects a --compare-tables spec with a blank side", func(t *testing.T) {
		s := newBoundaryTestShell(t, Usecases{})
		s.argument.CompareTables = "left,"
		if _, _, err := s.resolveCompareTables(context.Background()); err == nil {
			t.Fatal("want error for a blank right side, got nil")
		}
	})

	t.Run("reports a --compare-tables name that does not resolve", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		// resolveTableNameCI queries the master tables; an empty result means the
		// named table does not exist.
		query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(covErrEmptyTable(), nil).AnyTimes()
		s := newBoundaryTestShell(t, Usecases{query: query})
		s.argument.CompareTables = "left,right"
		if _, _, err := s.resolveCompareTables(context.Background()); err == nil {
			t.Fatal("want error for an unresolvable compare table, got nil")
		}
	})
}
