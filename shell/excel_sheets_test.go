package shell

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/xuri/excelize/v2"
)

// sheetSpec is one sheet of a workbook built for these tests.
type sheetSpec struct {
	name       string
	hidden     bool
	veryHidden bool
	value      string
}

// writeWorkbook builds a workbook holding exactly these sheets, in this order,
// each with one header cell and one data row.
//
// The workbook is generated rather than committed so the visibility each test
// depends on is stated in the test. A committed file could be named "hidden"
// and hold nothing hidden, and every assertion below would keep passing.
func writeWorkbook(t *testing.T, dir, name string, sheets ...sheetSpec) string {
	t.Helper()
	const defaultSheet = "Sheet1"

	f := excelize.NewFile()
	for _, sheet := range sheets {
		if sheet.name != defaultSheet {
			if _, err := f.NewSheet(sheet.name); err != nil {
				t.Fatalf("new sheet %q: %v", sheet.name, err)
			}
		}
		if err := f.SetCellValue(sheet.name, "A1", "v"); err != nil {
			t.Fatalf("write header of %q: %v", sheet.name, err)
		}
		if err := f.SetCellValue(sheet.name, "A2", sheet.value); err != nil {
			t.Fatalf("write row of %q: %v", sheet.name, err)
		}
	}
	if sheets[0].name != defaultSheet {
		if err := f.DeleteSheet(defaultSheet); err != nil {
			t.Fatalf("delete the default sheet: %v", err)
		}
	}

	// Excel refuses a workbook with nothing shown, so the active sheet moves onto
	// one that stays shown before anything is hidden.
	for _, sheet := range sheets {
		if sheet.hidden || sheet.veryHidden {
			continue
		}
		index, err := f.GetSheetIndex(sheet.name)
		if err != nil {
			t.Fatalf("index of %q: %v", sheet.name, err)
		}
		f.SetActiveSheet(index)
		break
	}
	for _, sheet := range sheets {
		switch {
		case sheet.veryHidden:
			if err := f.SetSheetVisible(sheet.name, false, true); err != nil {
				t.Fatalf("very-hide %q: %v", sheet.name, err)
			}
		case sheet.hidden:
			if err := f.SetSheetVisible(sheet.name, false); err != nil {
				t.Fatalf("hide %q: %v", sheet.name, err)
			}
		}
	}

	path := filepath.Join(dir, name)
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("save %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
	return path
}

// mixedWorkbook is the workbook these tests share: shown, hidden, very hidden,
// shown again. The trailing shown sheet is what fails if filtering reorders.
func mixedWorkbook(t *testing.T, dir string) string {
	t.Helper()
	return writeWorkbook(t, dir, "book.xlsx",
		sheetSpec{name: "Visible", value: "shown"},
		sheetSpec{name: "Internal", hidden: true, value: "hidden"},
		sheetSpec{name: "Secret", veryHidden: true, value: "very-hidden"},
		sheetSpec{name: "Summary", value: "shown-too"},
	)
}

// runShell runs a shell built from args, returning what it wrote to stdout and
// stderr. The two are captured separately because keeping them apart is half of
// what these tests assert.
func runShell(t *testing.T, args []string) (stdout, stderr string, err error) {
	t.Helper()
	shell, cleanup, err := newShell(t, args)
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	backupOut, backupErr := config.Stdout, config.Stderr
	var out, errOut strings.Builder
	config.Stdout, config.Stderr = &out, &errOut
	defer func() { config.Stdout, config.Stderr = backupOut, backupErr }()

	shell.isTTY = func() bool { return false }
	err = shell.Run(context.Background())
	return out.String(), errOut.String(), err
}

// TestExcelImport_LeavesHiddenSheetsOut is the default, and the reason the rest
// of this file exists: a workbook contributes the tables its shown sheets make,
// and nothing else.
func TestExcelImport_LeavesHiddenSheetsOut(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	stdout, _, err := runShell(t, []string{"sqly", "--output-format", "csv",
		"--sql", "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"book_Visible", "book_Summary"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not hold %s: %q", want, stdout)
		}
	}
	for _, unwanted := range []string{"book_Internal", "book_Secret"} {
		if strings.Contains(stdout, unwanted) {
			t.Errorf("stdout holds %s, which the workbook hides: %q", unwanted, stdout)
		}
	}
}

// TestExcelImport_IncludeHiddenSheetsImportsBothKinds pins the opt-in, and pins
// that it covers hidden and very hidden alike — the claim sqly makes about not
// telling them apart.
func TestExcelImport_IncludeHiddenSheetsImportsBothKinds(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	stdout, _, err := runShell(t, []string{"sqly", "--include-hidden-sheets", "--output-format", "csv",
		"--sql", "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"book_Visible", "book_Internal", "book_Secret", "book_Summary"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not hold %s: %q", want, stdout)
		}
	}
}

// TestExcelImport_SkipNoticeGoesToStderrAsACount covers the notice a user needs
// to explain a table they expected and did not get, and the two things it must
// not do: reach stdout, or publish the names hiding a sheet was meant to keep
// out of sight.
func TestExcelImport_SkipNoticeGoesToStderrAsACount(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	stdout, stderr, err := runShell(t, []string{"sqly", "--output-format", "csv",
		"--sql", "SELECT v FROM book_Visible", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, want := range []string{"Skipped 2 hidden sheets", "book.xlsx", "--include-hidden-sheets"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not hold %q: %q", want, stderr)
		}
	}
	if strings.Contains(stdout, "Skipped") {
		t.Errorf("the notice reached stdout, which carries the result: %q", stdout)
	}
	for _, name := range []string{"Internal", "Secret"} {
		if strings.Contains(stderr, name) {
			t.Errorf("the notice names the hidden sheet %q; it is meant to be a count: %q", name, stderr)
		}
	}
}

// TestExcelImport_NoNoticeWhenNothingWasSkipped keeps the notice from becoming
// noise on every workbook.
func TestExcelImport_NoNoticeWhenNothingWasSkipped(t *testing.T) {
	path := writeWorkbook(t, t.TempDir(), "plain.xlsx",
		sheetSpec{name: "One", value: "a"},
		sheetSpec{name: "Two", value: "b"},
	)

	_, stderr, err := runShell(t, []string{"sqly", "--output-format", "csv",
		"--sql", "SELECT v FROM plain_One", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(stderr, "Skipped") {
		t.Errorf("a workbook with nothing hidden produced a notice: %q", stderr)
	}
}

// TestExcelImport_NoNoticeWhenHiddenSheetsWereImported checks the other silence:
// a run that asked for the hidden sheets skipped nothing to report.
func TestExcelImport_NoNoticeWhenHiddenSheetsWereImported(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	_, stderr, err := runShell(t, []string{"sqly", "--include-hidden-sheets", "--output-format", "csv",
		"--sql", "SELECT v FROM book_Internal", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(stderr, "Skipped") {
		t.Errorf("--include-hidden-sheets still reported a skip: %q", stderr)
	}
}

// TestExcelImport_SingularNoticeForOneSheet is a wording check, and a small one,
// but "Skipped 1 hidden sheets" is the kind of thing a user reads as a bug in
// everything else the tool says.
func TestExcelImport_SingularNoticeForOneSheet(t *testing.T) {
	path := writeWorkbook(t, t.TempDir(), "one.xlsx",
		sheetSpec{name: "Shown", value: "a"},
		sheetSpec{name: "Tucked", hidden: true, value: "b"},
	)

	_, stderr, err := runShell(t, []string{"sqly", "--output-format", "csv",
		"--sql", "SELECT v FROM one_Shown", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stderr, "Skipped 1 hidden sheet in") {
		t.Errorf("stderr should say one sheet in the singular: %q", stderr)
	}
	if !strings.Contains(stderr, "import it.") {
		t.Errorf("stderr should refer to the one skipped sheet as it: %q", stderr)
	}
}

// TestExcelImport_CollisionIsJudgedAfterFiltering is the rule the two features
// have to agree on. Two sheets that want one table are a refusal — but only when
// both are loaded, because a sheet nobody reads cannot take a name from one that
// is read.
func TestExcelImport_CollisionIsJudgedAfterFiltering(t *testing.T) {
	newBook := func(t *testing.T) string {
		t.Helper()
		return writeWorkbook(t, t.TempDir(), "clash.xlsx",
			sheetSpec{name: "Q1 sales", value: "shown"},
			sheetSpec{name: "Q1.sales", hidden: true, value: "hidden"},
		)
	}

	t.Run("the hidden half does not collide while it is not loaded", func(t *testing.T) {
		stdout, _, err := runShell(t, []string{"sqly", "--output-format", "csv",
			"--sql", "SELECT v FROM clash_Q1_sales", newBook(t)})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !strings.Contains(stdout, "shown") {
			t.Errorf("stdout = %q, want the shown sheet's row", stdout)
		}
	})

	t.Run("--include-hidden-sheets brings it into the check and the workbook is refused", func(t *testing.T) {
		_, _, err := runShell(t, []string{"sqly", "--include-hidden-sheets",
			"--sql", "SELECT 1", newBook(t)})
		if err == nil {
			t.Fatal("Run succeeded on a workbook whose loaded sheets share a table name")
		}
		for _, want := range []string{"Q1 sales", "Q1.sales"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q should name %s", err, want)
			}
		}
	})
}

// TestInspect_ReportsEverySheetWithItsVisibility covers the report that is the
// only place a hidden sheet is named, field by field: what the workbook says,
// what this run did, and the table when there is one.
func TestInspect_ReportsEverySheetWithItsVisibility(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	for _, tt := range []struct {
		name           string
		args           []string
		wantImported   map[string]bool
		wantTable      map[string]string
		wantVisibility map[string]bool
	}{
		{
			name: "by default only the shown sheets are imported",
			args: []string{"sqly", "--inspect", path},
			wantImported: map[string]bool{
				"Visible": true, "Internal": false, "Secret": false, "Summary": true,
			},
			wantTable: map[string]string{
				"Visible": "book_Visible", "Internal": "", "Secret": "", "Summary": "book_Summary",
			},
			wantVisibility: map[string]bool{
				"Visible": true, "Internal": false, "Secret": false, "Summary": true,
			},
		},
		{
			name: "--include-hidden-sheets changes imported and leaves visible alone",
			args: []string{"sqly", "--inspect", "--include-hidden-sheets", path},
			wantImported: map[string]bool{
				"Visible": true, "Internal": true, "Secret": true, "Summary": true,
			},
			wantTable: map[string]string{
				"Visible": "book_Visible", "Internal": "book_Internal",
				"Secret": "book_Secret", "Summary": "book_Summary",
			},
			// visible is a property of the file, so the flag must not touch it.
			wantVisibility: map[string]bool{
				"Visible": true, "Internal": false, "Secret": false, "Summary": true,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runShell(t, tt.args)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			var report struct {
				Tables      []json.RawMessage `json:"tables"`
				ExcelSheets []struct {
					Source   string `json:"source"`
					Name     string `json:"name"`
					Visible  bool   `json:"visible"`
					Imported bool   `json:"imported"`
					Table    string `json:"table"`
				} `json:"excel_sheets"`
			}
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("parse the report: %v\n%s", err, stdout)
			}

			// Order is part of the contract: the workbook's own sheet order.
			wantOrder := []string{"Visible", "Internal", "Secret", "Summary"}
			if len(report.ExcelSheets) != len(wantOrder) {
				t.Fatalf("report holds %d sheets, want %d: %s", len(report.ExcelSheets), len(wantOrder), stdout)
			}
			for i, want := range wantOrder {
				got := report.ExcelSheets[i]
				if got.Name != want {
					t.Errorf("sheet %d is %q, want %q", i, got.Name, want)
					continue
				}
				if got.Visible != tt.wantVisibility[want] {
					t.Errorf("sheet %q reports visible=%t, want %t", want, got.Visible, tt.wantVisibility[want])
				}
				if got.Imported != tt.wantImported[want] {
					t.Errorf("sheet %q reports imported=%t, want %t", want, got.Imported, tt.wantImported[want])
				}
				if got.Table != tt.wantTable[want] {
					t.Errorf("sheet %q reports table=%q, want %q", want, got.Table, tt.wantTable[want])
				}
				if !strings.Contains(got.Source, "book.xlsx") {
					t.Errorf("sheet %q reports source=%q, want the workbook it came from", want, got.Source)
				}
			}
			if len(report.Tables) == 0 {
				t.Error("the report lost its tables; excel_sheets is meant to be additive")
			}
		})
	}
}

// TestInspect_NamesAWorkbookTheSameWayInBothFields pins the join a consumer of
// the report depends on: excel_sheets[].source and the tables[].source of the
// tables that workbook produced are the same string. They used to differ —
// tables were made absolute and sheets kept whatever was typed — so the two
// fields named one file in two ways and nothing could match them up.
//
// It also covers the consequence of keying records on the typed path: a workbook
// imported twice under two spellings became two sources in the report.
func TestInspect_NamesAWorkbookTheSameWayInBothFields(t *testing.T) {
	dir := t.TempDir()
	// The workdir is resolved because macOS reaches its temp directories through
	// a symlink, while the absolute path sqly computes comes from the working
	// directory the kernel reports. Without this the two spellings below would
	// differ for a reason that has nothing to do with what is being tested.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	path := mixedWorkbook(t, dir)
	t.Chdir(dir)

	s, cleanup, err := newShell(t, []string{"sqly", "--inspect", path})
	if err != nil {
		t.Fatalf("newShell: %v", err)
	}
	defer cleanup()

	s.recordTableSources(context.Background(), []string{"book_Visible"}, "book.xlsx")
	// The same workbook, reached twice by two different spellings.
	s.recordExcelSheets(path, path)
	s.recordExcelSheets(path, "book.xlsx")

	reports := s.excelSheetReports()
	if len(reports) == 0 {
		t.Fatal("no sheets were reported for a workbook that was recorded twice")
	}
	want := s.tableSources["book_Visible"]
	for _, report := range reports {
		if report.Source != want {
			t.Errorf("sheet %q reports source=%q, want the table's source %q", report.Name, report.Source, want)
		}
	}
	// Four sheets, recorded twice: a report holding eight means the two spellings
	// were kept as two workbooks.
	const sheetsInTheWorkbook = 4
	if len(reports) != sheetsInTheWorkbook {
		t.Errorf("the report holds %d sheets, want %d; the two spellings were not recognized as one workbook",
			len(reports), sheetsInTheWorkbook)
	}
}

// TestInspect_OmitsExcelSheetsWithoutAWorkbook is the additive half of the
// contract: a consumer reading only tables must see what it always saw.
func TestInspect_OmitsExcelSheetsWithoutAWorkbook(t *testing.T) {
	dir := t.TempDir()
	csv := writeCSV(t, dir, "rows.csv", "a,b\n1,2\n")

	stdout, _, err := runShell(t, []string{"sqly", "--inspect", csv})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(stdout, "excel_sheets") {
		t.Errorf("a run with no workbook still reported excel_sheets: %q", stdout)
	}
}

// TestInspect_StaysQuietOnStderrWhenSheetsWereSkipped checks the notice is not
// duplicated into a report that already names every sheet, and that stdout holds
// the report alone.
func TestInspect_StaysQuietOnStderrWhenSheetsWereSkipped(t *testing.T) {
	path := mixedWorkbook(t, t.TempDir())

	stdout, stderr, err := runShell(t, []string{"sqly", "--inspect", path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Contains(stderr, "Skipped") {
		t.Errorf("--inspect repeated the skip notice its report already covers: %q", stderr)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdout))) {
		t.Errorf("stdout is not one JSON document: %q", stdout)
	}
}
