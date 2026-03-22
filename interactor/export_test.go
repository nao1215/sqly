package interactor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/infrastructure/persistence"
	"github.com/nao1215/sqly/usecase"
)

// newTestExportInteractor creates an ExportUsecase backed by real persistence
// repositories. This avoids repeating the 5-line setup in every test function.
func newTestExportInteractor() usecase.ExportUsecase {
	return NewExportInteractor(
		persistence.NewCSVRepository(),
		persistence.NewTSVRepository(),
		persistence.NewLTSVRepository(),
		persistence.NewExcelRepository(),
		persistence.NewFileRepository(),
	)
}

func TestExportInteractor_DumpTable_CSV(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"name", "age", "city"}), []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
		model.NewRecord([]string{"Jane", "30", "Los Angeles"}),
	})

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.csv")

	if err := exp.DumpTable(outputFile, table, model.ExportCSV); err != nil {
		t.Fatalf("DumpTable CSV failed: %v", err)
	}

	content, err := os.ReadFile(outputFile) //nolint:gosec // test file with controlled path
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "name,age,city") {
		t.Errorf("Expected CSV header, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "John,25,New York") {
		t.Errorf("Expected first record, got: %s", contentStr)
	}
}

func TestExportInteractor_DumpTable_TSV(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"name", "age", "city"}), []model.Record{
		model.NewRecord([]string{"John", "25", "New York"}),
	})

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.tsv")

	if err := exp.DumpTable(outputFile, table, model.ExportTSV); err != nil {
		t.Fatalf("DumpTable TSV failed: %v", err)
	}

	content, err := os.ReadFile(outputFile) //nolint:gosec // test file with controlled path
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "name\tage\tcity") {
		t.Errorf("Expected TSV header with tabs, got: %s", string(content))
	}
}

func TestExportInteractor_DumpTable_LTSV(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"name", "age"}), []model.Record{
		model.NewRecord([]string{"John", "25"}),
	})

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.ltsv")

	if err := exp.DumpTable(outputFile, table, model.ExportLTSV); err != nil {
		t.Fatalf("DumpTable LTSV failed: %v", err)
	}

	content, err := os.ReadFile(outputFile) //nolint:gosec // test file with controlled path
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "name:John") {
		t.Errorf("Expected LTSV format, got: %s", string(content))
	}
}

func TestExportInteractor_DumpTable_Excel(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test_sheet", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.NewRecord([]string{"1", "Gina"}),
	})

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.xlsx")

	if err := exp.DumpTable(outputFile, table, model.ExportExcel); err != nil {
		t.Fatalf("DumpTable Excel failed: %v", err)
	}

	// Verify Excel file was created (starts with PK for zip format)
	content := make([]byte, 2)
	file, err := os.Open(outputFile) //nolint:gosec // test file with controlled path
	if err != nil {
		t.Fatalf("Failed to open Excel file: %v", err)
	}
	defer file.Close()

	if _, err := file.Read(content); err != nil {
		t.Fatalf("Failed to read Excel file header: %v", err)
	}

	if content[0] != 'P' || content[1] != 'K' {
		t.Errorf("Expected Excel file to start with PK, got: %v", content)
	}
}

func TestExportInteractor_DumpTable_Markdown(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.NewRecord([]string{"1", "Alice"}),
	})

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.md")

	if err := exp.DumpTable(outputFile, table, model.ExportMarkdown); err != nil {
		t.Fatalf("DumpTable Markdown failed: %v", err)
	}

	content, err := os.ReadFile(outputFile) //nolint:gosec // test file with controlled path
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "id") || !strings.Contains(string(content), "name") {
		t.Errorf("Expected markdown table content, got: %s", string(content))
	}
}

func TestExportInteractor_DumpTable_DefaultFormat(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"id"}), nil)

	tempDir := t.TempDir()
	outputFile := filepath.Join(tempDir, "output.txt")

	// Use an invalid format value to trigger default (CSV) path
	if err := exp.DumpTable(outputFile, table, model.ExportFormat(99)); err != nil {
		t.Fatalf("DumpTable default format failed: %v", err)
	}
}

func TestExportInteractor_DumpTable_InvalidPath(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()

	table := model.NewTable("test", model.NewHeader([]string{"id"}), nil)

	err := exp.DumpTable("/nonexistent/directory/file.csv", table, model.ExportCSV)
	if err == nil {
		t.Fatal("Expected error when dumping to invalid path")
	}
}
