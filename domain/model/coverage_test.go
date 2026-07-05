package model

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// covMdlFailingWriter is an io.Writer that always fails. It is used to exercise
// the error paths of the delimited writers.
type covMdlFailingWriter struct{}

func (covMdlFailingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("covMdl: forced write failure")
}

// TestWriteDelimitedThroughPrint covers writeDelimited via printCSV/printTSV for
// values that require quoting (delimiter, quote, newline) and for an empty result
// set, verifying the output round-trips as delimited data.
func TestWriteDelimitedThroughPrint(t *testing.T) {
	t.Parallel()

	t.Run("csv quotes values with commas, quotes and newlines", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"name", "note"}), []Record{
			NewRecord([]string{"a,b", "line1\nline2"}),
			NewRecord([]string{`he said "hi"`, ""}),
		})
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeCSV); err != nil {
			t.Fatalf("Print(CSV) error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, `"a,b"`) {
			t.Errorf("expected comma value to be quoted, got: %q", got)
		}
		if !strings.Contains(got, `"line1`) {
			t.Errorf("expected newline value to be quoted, got: %q", got)
		}
		if !strings.Contains(got, `"he said ""hi"""`) {
			t.Errorf("expected embedded quotes to be escaped, got: %q", got)
		}
	})

	t.Run("tsv writes header only for empty records", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a", "b"}), nil)
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeTSV); err != nil {
			t.Fatalf("Print(TSV) error = %v", err)
		}
		if got := buf.String(); got != "a\tb\n" {
			t.Errorf("empty TSV = %q, want %q", got, "a\tb\n")
		}
	})

	t.Run("write failure surfaces as error", func(t *testing.T) {
		t.Parallel()
		// A column name longer than the csv writer's internal buffer forces a flush
		// during Write, so the underlying writer error surfaces immediately as the
		// header write error rather than being deferred to Flush.
		bigHeader := strings.Repeat("x", 8192)
		table := NewTable("t", NewHeader([]string{bigHeader}), []Record{
			NewRecord([]string{"v"}),
		})
		if err := table.Print(covMdlFailingWriter{}, PrintModeCSV); err == nil {
			t.Fatal("Print(CSV) to failing writer = nil error, want error")
		}
	})
}

// TestPrintJSONEdgeCases covers printJSON for an empty result set, duplicate
// column names, and a normal multi-row render.
func TestPrintJSONEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty records print an empty array", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a"}), nil)
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeJSON); err != nil {
			t.Fatalf("Print(JSON) error = %v", err)
		}
		if got := strings.TrimSpace(buf.String()); got != "[]" {
			t.Errorf("empty JSON = %q, want %q", got, "[]")
		}
	})

	t.Run("duplicate columns are rejected", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a", "a"}), []Record{
			NewRecord([]string{"1", "2"}),
		})
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeJSON); err == nil {
			t.Fatal("Print(JSON) = nil error, want error for duplicate columns")
		}
	})

	t.Run("multi row render with special characters", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"name", "val"}), []Record{
			NewRecord([]string{"quote\"", "007"}),
			NewRecord([]string{"tab\there", ""}),
		})
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeJSON); err != nil {
			t.Fatalf("Print(JSON) error = %v", err)
		}
		got := buf.String()
		if !strings.HasPrefix(strings.TrimSpace(got), "[") || !strings.HasSuffix(strings.TrimSpace(got), "]") {
			t.Errorf("JSON output not wrapped in an array: %q", got)
		}
		if !strings.Contains(got, `"007"`) {
			t.Errorf("expected leading-zero value preserved as string, got: %q", got)
		}
	})

	t.Run("write failure surfaces as error", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a"}), []Record{
			NewRecord([]string{"1"}),
		})
		if err := table.Print(covMdlFailingWriter{}, PrintModeJSON); err == nil {
			t.Fatal("Print(JSON) to failing writer = nil error, want error")
		}
	})
}

// TestPrintNDJSONEdgeCases covers printNDJSON for an empty result set (empty
// stream), duplicate column names, and a normal render of one object per line.
func TestPrintNDJSONEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty records print nothing", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a"}), nil)
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeNDJSON); err != nil {
			t.Fatalf("Print(NDJSON) error = %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("empty NDJSON = %q, want empty stream", buf.String())
		}
	})

	t.Run("duplicate columns are rejected", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a", "a"}), []Record{
			NewRecord([]string{"1", "2"}),
		})
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeNDJSON); err == nil {
			t.Fatal("Print(NDJSON) = nil error, want error for duplicate columns")
		}
	})

	t.Run("one object per line", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a", "b"}), []Record{
			NewRecord([]string{"1", "x"}),
			NewRecord([]string{"2", "y"}),
		})
		var buf bytes.Buffer
		if err := table.Print(&buf, PrintModeNDJSON); err != nil {
			t.Fatalf("Print(NDJSON) error = %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("NDJSON produced %d lines, want 2: %q", len(lines), buf.String())
		}
		for _, line := range lines {
			if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
				t.Errorf("NDJSON line is not a JSON object: %q", line)
			}
		}
	})

	t.Run("write failure surfaces as error", func(t *testing.T) {
		t.Parallel()
		table := NewTable("t", NewHeader([]string{"a"}), []Record{
			NewRecord([]string{"1"}),
		})
		if err := table.Print(covMdlFailingWriter{}, PrintModeNDJSON); err == nil {
			t.Fatal("Print(NDJSON) to failing writer = nil error, want error")
		}
	})
}
