package shell

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

// covCmpIdentityImporter returns a MockImportUsecase whose identifier helpers are
// pass-through, so query strings a shell builds are predictable in a mock-driven
// test. QuoteIdentifier double-quotes and SanitizeForSQL returns the name as-is.
func covCmpIdentityImporter(ctrl *gomock.Controller) *mock.MockImportUsecase {
	imp := mock.NewMockImportUsecase(ctrl)
	imp.EXPECT().QuoteIdentifier(gomock.Any()).
		DoAndReturn(func(s string) string { return `"` + s + `"` }).AnyTimes()
	imp.EXPECT().SanitizeForSQL(gomock.Any()).
		DoAndReturn(func(s string) string { return s }).AnyTimes()
	return imp
}

// covCmpImportedShell builds a real shell backed by the in-memory database and
// imports the given "table name -> CSV content" fixtures, returning the shell and
// its cleanup. It is used for compare/inspect error paths that a real SQLite
// session triggers (a missing table, an unresolvable key column, an invalid
// CREATE INDEX target).
func covCmpImportedShell(t *testing.T, files map[string]string) (*Shell, func()) {
	t.Helper()
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	dir := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, content := range files {
		paths = append(paths, writeCSV(t, dir, name+".csv", content))
	}
	if err := shell.commands.importCommand(context.Background(), shell, paths); err != nil {
		cleanup()
		t.Fatalf("import: %v", err)
	}
	return shell, cleanup
}

func TestOutputModeFlagName_NamesTypedVariants(t *testing.T) {
	t.Parallel()

	t.Run("nil output yields empty name", func(t *testing.T) {
		t.Parallel()
		if got := outputModeFlagName(nil); got != "" {
			t.Errorf("outputModeFlagName(nil) = %q, want empty", got)
		}
	})

	t.Run("json-typed names the typed json flag", func(t *testing.T) {
		t.Parallel()
		o := &config.Output{JSONTyped: true, Mode: model.PrintModeJSON}
		if got := outputModeFlagName(o); got != outputModeJSONTyped {
			t.Errorf("outputModeFlagName = %q, want %q", got, outputModeJSONTyped)
		}
	})

	t.Run("ndjson-typed names the typed ndjson flag", func(t *testing.T) {
		t.Parallel()
		o := &config.Output{JSONTyped: true, Mode: model.PrintModeNDJSON}
		if got := outputModeFlagName(o); got != outputModeNDJSONTyped {
			t.Errorf("outputModeFlagName = %q, want %q", got, outputModeNDJSONTyped)
		}
	})

	t.Run("typed flag on a non-json mode falls back to the base mode name", func(t *testing.T) {
		t.Parallel()
		o := &config.Output{JSONTyped: true, Mode: model.PrintModeCSV}
		if got := outputModeFlagName(o); got != model.PrintModeCSV.String() {
			t.Errorf("outputModeFlagName = %q, want %q", got, model.PrintModeCSV.String())
		}
	})

	t.Run("plain mode returns the base mode name", func(t *testing.T) {
		t.Parallel()
		o := &config.Output{Mode: model.PrintModeCSV}
		if got := outputModeFlagName(o); got != model.PrintModeCSV.String() {
			t.Errorf("outputModeFlagName = %q, want %q", got, model.PrintModeCSV.String())
		}
	})
}

func TestValidateCompareFlags_RejectsConflicts(t *testing.T) {
	t.Parallel()

	t.Run("no compare flag is always accepted", func(t *testing.T) {
		t.Parallel()
		s := &Shell{argument: &config.Arg{CompareFlag: false, Query: "SELECT 1"}}
		if err := s.validateCompareFlags(); err != nil {
			t.Errorf("validateCompareFlags without --compare = %v, want nil", err)
		}
	})

	cases := []struct {
		name string
		arg  *config.Arg
	}{
		{"with --inspect", &config.Arg{CompareFlag: true, InspectFlag: true}},
		{"with --sql", &config.Arg{CompareFlag: true, Query: "SELECT 1"}},
		{"with --sql-file", &config.Arg{CompareFlag: true, SQLFilePath: "q.sql"}},
		{"with --output", &config.Arg{CompareFlag: true, Output: &config.Output{FilePath: "out.csv"}}},
		{"with --save", &config.Arg{CompareFlag: true, SaveInPlace: true}},
		{"with --save-dir", &config.Arg{CompareFlag: true, SaveDir: "dir"}},
		{"with an output mode flag", &config.Arg{CompareFlag: true, Output: &config.Output{Mode: model.PrintModeCSV}}},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is rejected", func(t *testing.T) {
			t.Parallel()
			s := &Shell{argument: tc.arg}
			if err := s.validateCompareFlags(); err == nil {
				t.Errorf("validateCompareFlags %s = nil, want an error", tc.name)
			}
		})
	}

	t.Run("compare alone with a table output mode is accepted", func(t *testing.T) {
		t.Parallel()
		s := &Shell{argument: &config.Arg{CompareFlag: true, Output: &config.Output{Mode: model.PrintModeTable}}}
		if err := s.validateCompareFlags(); err != nil {
			t.Errorf("validateCompareFlags for a clean --compare = %v, want nil", err)
		}
	})
}

func TestResolveTableNameCI_NotFound(t *testing.T) {
	t.Parallel()

	t.Run("a query error resolves to not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

		s := &Shell{usecases: Usecases{query: query}}
		if name, ok := s.resolveTableNameCI(context.Background(), "x"); ok || name != "" {
			t.Errorf("resolveTableNameCI on a query error = (%q, %v), want (\"\", false)", name, ok)
		}
	})

	t.Run("no matching record resolves to not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(model.NewTable("", nil, nil), nil)

		s := &Shell{usecases: Usecases{query: query}}
		if name, ok := s.resolveTableNameCI(context.Background(), "x"); ok || name != "" {
			t.Errorf("resolveTableNameCI with no rows = (%q, %v), want (\"\", false)", name, ok)
		}
	})
}

func TestBuildCompareReport_ErrorPaths(t *testing.T) {
	t.Run("a missing left table surfaces the row-count error", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{
			"present": "id,name\n1,Alice\n",
		})
		defer cleanup()
		if _, err := shell.buildCompareReport(context.Background(), "ghost", "present", ""); err == nil {
			t.Error("buildCompareReport with a missing left table returned nil, want an error")
		}
	})

	t.Run("a key absent from the right table is rejected", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{
			"cleft":  "id,name\n1,Alice\n",
			"cright": "other,name\n1,Alice\n",
		})
		defer cleanup()
		// The key "id" resolves on the left but not on the right, so the keyed diff
		// must fail while resolving the right key column.
		if _, err := shell.buildCompareReport(context.Background(), "cleft", "cright", "id"); err == nil {
			t.Error("buildCompareReport with a right-side missing key returned nil, want an error")
		}
	})
}

func TestCreateCompareKeyIndexes_ErrorPaths(t *testing.T) {
	t.Run("an invalid left table fails index creation", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{
			"idxbase": "id,score\n1,10\n",
		})
		defer cleanup()
		if _, err := shell.createCompareKeyIndexes(context.Background(), "ghost", "ghost2", "id", "id"); err == nil {
			t.Error("createCompareKeyIndexes on a missing left table returned nil, want an error")
		}
	})

	t.Run("an invalid right table drops the left index and fails", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{
			"idxleft": "id,score\n1,10\n",
		})
		defer cleanup()
		// The left index is created on the real table, then the right CREATE fails on
		// the missing table, so the cleanup drops the left index before returning.
		if _, err := shell.createCompareKeyIndexes(context.Background(), "idxleft", "ghost", "id", "id"); err == nil {
			t.Error("createCompareKeyIndexes on a missing right table returned nil, want an error")
		}
	})
}

func TestDropCompareKeyIndex_WarnsOnFailure(t *testing.T) {
	shell, cleanup := covCmpImportedShell(t, map[string]string{
		"anytable": "id\n1\n",
	})
	defer cleanup()

	stderr := captureStderr(t, func() {
		shell.dropCompareKeyIndex(context.Background(), "no_such_compare_index")
	})
	if !strings.Contains(stderr, "warning") {
		t.Errorf("dropCompareKeyIndex of a missing index should warn on stderr, got %q", stderr)
	}
}

func TestRecordToCompareRow_SkipsShortAndMarksNull(t *testing.T) {
	t.Parallel()
	cols := []inspectColumn{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	record := []string{"x", ""}  // shorter than cols: "c" has no cell
	nulls := []bool{false, true} // "b" is a SQL NULL
	row := recordToCompareRow(cols, record, nulls)

	if row["a"] == nil || *row["a"] != "x" {
		t.Errorf("column a = %v, want pointer to \"x\"", row["a"])
	}
	if row["b"] != nil {
		t.Errorf("column b should be NULL (nil), got %v", row["b"])
	}
	if _, ok := row["c"]; ok {
		t.Errorf("column c should be absent (record too short), got %v", row["c"])
	}
}

func TestSameColumnNames_DetectsDifferences(t *testing.T) {
	t.Parallel()
	col := func(name string) inspectColumn { return inspectColumn{Name: name} }

	t.Run("different lengths are not equal", func(t *testing.T) {
		t.Parallel()
		if sameColumnNames([]inspectColumn{col("a")}, []inspectColumn{col("a"), col("b")}) {
			t.Error("column lists of different length reported equal")
		}
	})

	t.Run("same length but different names are not equal", func(t *testing.T) {
		t.Parallel()
		if sameColumnNames([]inspectColumn{col("a"), col("b")}, []inspectColumn{col("a"), col("c")}) {
			t.Error("column lists with a differing name reported equal")
		}
	})

	t.Run("identical name sets are equal", func(t *testing.T) {
		t.Parallel()
		if !sameColumnNames([]inspectColumn{col("a"), col("b")}, []inspectColumn{col("b"), col("a")}) {
			t.Error("identical name sets reported not equal")
		}
	})
}

func TestRowMapsEqual_Rules(t *testing.T) {
	t.Parallel()
	ptr := func(s string) *string { return &s }

	t.Run("different lengths are not equal", func(t *testing.T) {
		t.Parallel()
		if rowMapsEqual(compareRow{"a": ptr("1")}, compareRow{"a": ptr("1"), "b": ptr("2")}) {
			t.Error("rows of different size reported equal")
		}
	})

	t.Run("a key missing on the right is not equal", func(t *testing.T) {
		t.Parallel()
		if rowMapsEqual(compareRow{"a": ptr("1")}, compareRow{"b": ptr("1")}) {
			t.Error("rows with disjoint keys reported equal")
		}
	})

	t.Run("two NULLs are equal", func(t *testing.T) {
		t.Parallel()
		if !rowMapsEqual(compareRow{"a": nil}, compareRow{"a": nil}) {
			t.Error("two NULLs on the same key reported not equal")
		}
	})

	t.Run("a NULL differs from a value", func(t *testing.T) {
		t.Parallel()
		if rowMapsEqual(compareRow{"a": nil}, compareRow{"a": ptr("1")}) {
			t.Error("NULL versus a value reported equal")
		}
	})

	t.Run("differing values are not equal", func(t *testing.T) {
		t.Parallel()
		if rowMapsEqual(compareRow{"a": ptr("1")}, compareRow{"a": ptr("2")}) {
			t.Error("differing values reported equal")
		}
	})

	t.Run("identical rows are equal", func(t *testing.T) {
		t.Parallel()
		if !rowMapsEqual(compareRow{"a": ptr("1"), "b": nil}, compareRow{"a": ptr("1"), "b": nil}) {
			t.Error("identical rows reported not equal")
		}
	})
}

func TestRenderCompareText_AllSections(t *testing.T) {
	t.Parallel()

	t.Run("identical schema without keyed rows", func(t *testing.T) {
		t.Parallel()
		r := compareReport{
			Left:     "l",
			Right:    "r",
			Schema:   compareSchema{Equal: true},
			RowCount: compareRowCount{Left: 2, Right: 2, Delta: 0},
		}
		out := renderCompareText(r)
		if !strings.Contains(out, "compare l -> r") {
			t.Errorf("missing header line: %q", out)
		}
		if !strings.Contains(out, "schema: identical") {
			t.Errorf("missing identical-schema line: %q", out)
		}
		if strings.Contains(out, "keyed by") {
			t.Errorf("keyed line printed without a rows section: %q", out)
		}
	})

	t.Run("changed schema with keyed rows renders every section", func(t *testing.T) {
		t.Parallel()
		r := compareReport{
			Left:  "l",
			Right: "r",
			Schema: compareSchema{
				Equal:            false,
				LeftOnlyColumns:  []string{"old"},
				RightOnlyColumns: []string{"new"},
				TypeChanges:      []compareColumnTypeChange{{Name: "id", LeftType: "INTEGER", RightType: "TEXT"}},
			},
			RowCount: compareRowCount{Left: 3, Right: 5, Delta: 2},
			Rows: &compareRows{
				Key:      "id",
				Added:    []compareRow{{}},
				Removed:  []compareRow{{}, {}},
				Modified: []compareModifiedRow{{Key: "1"}},
			},
		}
		out := renderCompareText(r)
		for _, want := range []string{
			"schema: changed",
			"columns only in l: old",
			"columns only in r: new",
			"type change id: INTEGER -> TEXT",
			"rows: 3 -> 5 (delta +2)",
			"keyed by id: 1 added, 2 removed, 1 modified",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("renderCompareText output missing %q:\n%s", want, out)
			}
		}
	})
}

func TestRunCompare_TextFormat(t *testing.T) {
	shell, cleanup := covCmpImportedShell(t, map[string]string{
		"tleft":  "id,name\n1,Alice\n2,Bob\n",
		"tright": "id,name\n1,Alice\n2,Bob\n",
	})
	defer cleanup()
	shell.argument.CompareFormat = outputFormatText

	out := captureStdout(t, func() {
		if _, _, err := shell.resolveCompareTables(context.Background()); err != nil {
			t.Fatalf("resolveCompareTables: %v", err)
		}
		report, err := shell.buildCompareReport(context.Background(), "tleft", "tright", "")
		if err != nil {
			t.Fatalf("buildCompareReport: %v", err)
		}
		_ = report
	})
	_ = out

	// Drive the full text path through runCompare so the text branch is exercised.
	text := captureStdout(t, func() {
		if err := shell.runCompare(context.Background()); err != nil {
			t.Fatalf("runCompare (text): %v", err)
		}
	})
	if !strings.Contains(text, "compare ") || !strings.Contains(text, "schema:") {
		t.Errorf("runCompare text output missing summary lines: %q", text)
	}
}

func TestInspect_ErrorPathsWithRealDB(t *testing.T) {
	t.Run("inspectRowCount on a missing table errors", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{"present": "a\n1\n"})
		defer cleanup()
		if _, err := shell.inspectRowCount(context.Background(), "ghost"); err == nil {
			t.Error("inspectRowCount on a missing table returned nil, want an error")
		}
	})

	t.Run("inspectSample on a missing table errors", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{"present": "a\n1\n"})
		defer cleanup()
		if _, err := shell.inspectSample(context.Background(), "ghost", 5); err == nil {
			t.Error("inspectSample on a missing table returned nil, want an error")
		}
	})

	t.Run("inspectTable on a missing table errors", func(t *testing.T) {
		shell, cleanup := covCmpImportedShell(t, map[string]string{"present": "a\n1\n"})
		defer cleanup()
		if _, err := shell.inspectTable(context.Background(), "ghost", 5); err == nil {
			t.Error("inspectTable on a missing table returned nil, want an error")
		}
	})
}

func TestInspectColumns_MockErrorAndShortRecord(t *testing.T) {
	t.Run("a PRAGMA query error is surfaced", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, q string) (*model.Table, error) {
				if strings.Contains(q, "PRAGMA") {
					return nil, errors.New("boom")
				}
				return model.NewTable("", nil, nil), nil // objectExists lookup
			}).AnyTimes()

		s := &Shell{usecases: Usecases{query: query, importer: covCmpIdentityImporter(ctrl)}}
		if _, err := s.inspectColumns(context.Background(), "t"); err == nil {
			t.Error("inspectColumns with a PRAGMA error returned nil, want an error")
		}
	})

	t.Run("a short PRAGMA record is skipped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, q string) (*model.Table, error) {
				if strings.Contains(q, "PRAGMA") {
					// Fewer than the 6 PRAGMA table_info columns: the row is skipped.
					return model.NewTable("", model.Header{"cid", "name", "type"},
						[]model.Record{{"0", "c", "TEXT"}}), nil
				}
				return model.NewTable("", nil, nil), nil
			}).AnyTimes()

		s := &Shell{usecases: Usecases{query: query, importer: covCmpIdentityImporter(ctrl)}}
		cols, err := s.inspectColumns(context.Background(), "t")
		if err != nil {
			t.Fatalf("inspectColumns: %v", err)
		}
		if len(cols) != 0 {
			t.Errorf("short PRAGMA record was not skipped, got %+v", cols)
		}
	})
}

func TestInspectRowCount_MockEdgeCases(t *testing.T) {
	t.Run("empty result yields zero", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(model.NewTable("", nil, nil), nil)

		s := &Shell{usecases: Usecases{query: query, importer: covCmpIdentityImporter(ctrl)}}
		got, err := s.inspectRowCount(context.Background(), "t")
		if err != nil {
			t.Fatalf("inspectRowCount: %v", err)
		}
		if got != 0 {
			t.Errorf("inspectRowCount on empty result = %d, want 0", got)
		}
	})

	t.Run("a non-numeric count is an error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		query := mock.NewMockQueryUsecase(ctrl)
		query.EXPECT().Query(gomock.Any(), gomock.Any()).
			Return(model.NewTable("", model.Header{"c"}, []model.Record{{"not-a-number"}}), nil)

		s := &Shell{usecases: Usecases{query: query, importer: covCmpIdentityImporter(ctrl)}}
		if _, err := s.inspectRowCount(context.Background(), "t"); err == nil {
			t.Error("inspectRowCount with a non-numeric count returned nil, want an error")
		}
	})
}

func TestRunInspect_TablesNameError(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return(nil, errors.New("boom"))

	s := &Shell{
		argument: &config.Arg{Output: &config.Output{Mode: model.PrintModeTable}},
		usecases: Usecases{metadata: metadata},
	}
	if err := s.runInspect(context.Background()); err == nil {
		t.Error("runInspect with a TablesName error returned nil, want an error")
	}
}

func TestTableCreateStatement_StoredSQLError(t *testing.T) {
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	query.EXPECT().Query(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, q string) (*model.Table, error) {
			if strings.Contains(q, "SELECT sql") {
				return nil, errors.New("boom") // storedCreateSQL master lookup fails
			}
			return model.NewTable("", nil, nil), nil // objectExists lookup: not found
		}).AnyTimes()

	s := &Shell{usecases: Usecases{query: query, importer: covCmpIdentityImporter(ctrl)}}
	if _, err := s.tableCreateStatement(context.Background(), "t"); err == nil {
		t.Error("tableCreateStatement with a stored-SQL query error returned nil, want an error")
	}
}

func TestInjectTempKeyword_Cases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain create table gains TEMP", "CREATE TABLE t (a)", "CREATE TEMP TABLE t (a)"},
		{"leading whitespace is preserved", "  \n\tCREATE TABLE t (a)", "  \n\tCREATE TEMP TABLE t (a)"},
		{"a non-create statement is unchanged", "SELECT 1", "SELECT 1"},
		{"an already TEMP statement is unchanged", "CREATE TEMP TABLE t (a)", "CREATE TEMP TABLE t (a)"},
		{"an already TEMPORARY statement is unchanged", "CREATE TEMPORARY TABLE t (a)", "CREATE TEMPORARY TABLE t (a)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := injectTempKeyword(tc.in); got != tc.want {
				t.Errorf("injectTempKeyword(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestObjectExists_QueryErrorIsFalse(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	query := mock.NewMockQueryUsecase(ctrl)
	query.EXPECT().Query(gomock.Any(), gomock.Any()).Return(nil, errors.New("boom"))

	s := &Shell{usecases: Usecases{query: query}}
	if s.objectExists(context.Background(), "x") {
		t.Error("objectExists on a query error = true, want false")
	}
}

func TestIsSchemaName_Cases(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"main":  true,
		"TEMP":  true,
		"temp":  true,
		"other": false,
		"":      false,
	}
	for prefix, want := range cases {
		if got := isSchemaName(prefix); got != want {
			t.Errorf("isSchemaName(%q) = %v, want %v", prefix, got, want)
		}
	}
}

func TestTablesCommand_ErrorPaths(t *testing.T) {
	t.Run("extra arguments are rejected", func(t *testing.T) {
		s := &Shell{}
		if err := NewCommands().tablesCommand(context.Background(), s, []string{"unexpected"}); err == nil {
			t.Error("tablesCommand with arguments returned nil, want an error")
		}
	})

	t.Run("a SchemaObjects error is propagated", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		metadata := mock.NewMockMetadataUsecase(ctrl)
		metadata.EXPECT().SchemaObjects(gomock.Any()).Return(nil, errors.New("boom"))

		st, err := newState(&config.Arg{Output: &config.Output{Mode: model.PrintModeTable}})
		if err != nil {
			t.Fatal(err)
		}
		s := &Shell{usecases: Usecases{metadata: metadata}, state: st}
		if err := NewCommands().tablesCommand(context.Background(), s, nil); err == nil {
			t.Error("tablesCommand with a SchemaObjects error returned nil, want an error")
		}
	})
}

func TestIsBareIdentifier_Cases(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"":     false,
		"abc":  true,
		"_x":   true,
		"a1_b": true,
		"1abc": false, // cannot start with a digit
		"a-b":  false, // hyphen is not allowed
		"a b":  false, // space is not allowed
	}
	for in, want := range cases {
		if got := isBareIdentifier(in); got != want {
			t.Errorf("isBareIdentifier(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestPrintTables_Empty(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	if err := printTables(&b, nil); err != nil {
		t.Fatalf("printTables(nil): %v", err)
	}
	if !strings.Contains(b.String(), "there is no table") {
		t.Errorf("printTables on an empty list = %q, want the empty-list hint", b.String())
	}
}
