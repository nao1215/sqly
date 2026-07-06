package persistence

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/xuri/excelize/v2"
)

// covPerNewHistoryRepo returns a history repository backed by its own in-memory
// database so subtests can run in parallel.
func covPerNewHistoryRepo(t *testing.T) (*historyRepository, func()) {
	t.Helper()
	historyDB, cleanup, err := config.NewInMemHistoryDB()
	if err != nil {
		t.Fatal(err)
	}
	repo, ok := NewHistoryRepository(historyDB).(*historyRepository)
	if !ok {
		t.Fatal("NewHistoryRepository did not return *historyRepository")
	}
	return repo, cleanup
}

func TestExcelRepository_Dump_SaveToMissingDirectoryReturnsError(t *testing.T) {
	t.Parallel()

	r := NewExcelRepository()
	table := model.NewTable("sheet", model.Header{"id"}, []model.Record{{"1"}})
	// SaveAs targets a directory that does not exist, so the write fails.
	path := filepath.Join(t.TempDir(), "no_such_dir", "out.xlsx")
	if err := r.Dump(path, table); err == nil {
		t.Error("expected error dumping to a missing directory, got nil")
	}
}

func TestExcelRepository_Dump_SanitizesInvalidSheetName(t *testing.T) {
	t.Parallel()

	r := NewExcelRepository()
	// Excel sheet names cannot contain ':'; the name is sanitized so the dump
	// succeeds instead of failing on NewSheet.
	table := model.NewTable("bad:name", model.Header{"id"}, []model.Record{{"1"}})
	path := filepath.Join(t.TempDir(), "out.xlsx")
	if err := r.Dump(path, table); err != nil {
		t.Fatalf("Dump failed for a punctuated sheet name: %v", err)
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sheets := f.GetSheetList()
	if len(sheets) != 1 {
		t.Fatalf("sheet count = %d, want 1", len(sheets))
	}
	if strings.ContainsAny(sheets[0], `:\/?*[]`) {
		t.Errorf("sheet name %q still contains a forbidden character", sheets[0])
	}
}

func TestHistoryRepository_Create_InvalidTableReturnsError(t *testing.T) {
	t.Parallel()

	r, cleanup := covPerNewHistoryRepo(t)
	defer cleanup()
	if err := r.CreateTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	// An empty table fails Valid() before any SQL runs.
	if err := r.Create(context.Background(), model.NewTable("", model.Header{}, []model.Record{})); err == nil {
		t.Error("expected error creating from an invalid table, got nil")
	}
}

func TestHistoryRepository_Create_WithoutTableReturnsError(t *testing.T) {
	t.Parallel()

	r, cleanup := covPerNewHistoryRepo(t)
	defer cleanup()
	// The history table was never created, so the INSERT fails.
	input := model.Histories{model.NewHistory(0, "SELECT 1")}.ToTable()
	if err := r.Create(context.Background(), input); err == nil {
		t.Error("expected error inserting without a history table, got nil")
	}
}

func TestHistoryRepository_List_ReturnsRowsInIdOrder(t *testing.T) {
	t.Parallel()

	r, cleanup := covPerNewHistoryRepo(t)
	defer cleanup()
	if err := r.CreateTable(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, req := range []string{"one", "two", "three"} {
		in := model.Histories{model.NewHistory(0, req)}.ToTable()
		if err := r.Create(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("history count = %d, want 3", len(got))
	}
	// List orders by id ASC, matching the insertion order.
	wantReq := []string{"one", "two", "three"}
	for i, h := range got {
		if h.Request != wantReq[i] || h.ID != i+1 {
			t.Errorf("history[%d] = {id:%d req:%q}, want {id:%d req:%q}", i, h.ID, h.Request, i+1, wantReq[i])
		}
	}
}

func TestLTSVRepository_Dump_WritesLabelValueTokens(t *testing.T) {
	t.Parallel()

	r := NewLTSVRepository()
	table := model.NewTable("t", model.Header{"a", "b"}, []model.Record{
		{"1", "x"},
		{"2", "y"},
	})
	var buf bytes.Buffer
	if err := r.Dump(&buf, table); err != nil {
		t.Fatalf("Dump error = %v, want nil", err)
	}
	got := buf.String()
	want := "a:1\tb:x\na:2\tb:y\n"
	if got != want {
		t.Errorf("Dump output = %q, want %q", got, want)
	}
}

func TestLTSVRepository_Dump_ValueWithTabReturnsError(t *testing.T) {
	t.Parallel()

	r := NewLTSVRepository()
	// A tab inside a value cannot be represented in LTSV and must be rejected.
	table := model.NewTable("t", model.Header{"a"}, []model.Record{{"has\ttab"}})
	var buf bytes.Buffer
	err := r.Dump(&buf, table)
	if err == nil {
		t.Fatal("expected error for a value containing a tab, got nil")
	}
	if !strings.Contains(err.Error(), "tab or newline") {
		t.Errorf("error = %v, want message about tab or newline", err)
	}
}

func TestLTSVRepository_Dump_InvalidHeaderReturnsError(t *testing.T) {
	t.Parallel()

	r := NewLTSVRepository()
	// A label containing ':' is not a writable LTSV label.
	table := model.NewTable("t", model.Header{"bad:label"}, []model.Record{{"1"}})
	var buf bytes.Buffer
	if err := r.Dump(&buf, table); err == nil {
		t.Error("expected error for an invalid LTSV header, got nil")
	}
}
