package persistence

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/xuri/excelize/v2"
)

func TestExcelRepository_Dump_SaveToMissingDirectoryReturnsError(t *testing.T) {
	t.Parallel()

	table := model.NewTable("sheet", model.Header{"id"}, []model.Record{{"1"}})
	// SaveAs targets a directory that does not exist, so the write fails.
	path := filepath.Join(t.TempDir(), "no_such_dir", "out.xlsx")
	if err := DumpExcel(path, table); err == nil {
		t.Error("expected error dumping to a missing directory, got nil")
	}
}

func TestExcelRepository_Dump_SanitizesInvalidSheetName(t *testing.T) {
	t.Parallel()

	// Excel sheet names cannot contain ':'; the name is sanitized so the dump
	// succeeds instead of failing on NewSheet.
	table := model.NewTable("bad:name", model.Header{"id"}, []model.Record{{"1"}})
	path := filepath.Join(t.TempDir(), "out.xlsx")
	if err := DumpExcel(path, table); err != nil {
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

func TestLTSVRepository_Dump_WritesLabelValueTokens(t *testing.T) {
	t.Parallel()

	table := model.NewTable("t", model.Header{"a", "b"}, []model.Record{
		{"1", "x"},
		{"2", "y"},
	})
	var buf bytes.Buffer
	if err := DumpLTSV(&buf, table); err != nil {
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

	// A tab inside a value cannot be represented in LTSV and must be rejected.
	table := model.NewTable("t", model.Header{"a"}, []model.Record{{"has\ttab"}})
	var buf bytes.Buffer
	err := DumpLTSV(&buf, table)
	if err == nil {
		t.Fatal("expected error for a value containing a tab, got nil")
	}
	if !strings.Contains(err.Error(), "tab or newline") {
		t.Errorf("error = %v, want message about tab or newline", err)
	}
}

func TestLTSVRepository_Dump_InvalidHeaderReturnsError(t *testing.T) {
	t.Parallel()

	// A label containing ':' is not a writable LTSV label.
	table := model.NewTable("t", model.Header{"bad:label"}, []model.Record{{"1"}})
	var buf bytes.Buffer
	if err := DumpLTSV(&buf, table); err == nil {
		t.Error("expected error for an invalid LTSV header, got nil")
	}
}
