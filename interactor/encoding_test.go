package interactor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
)

// TestDumpTableWritesTheSourceEncoding checks that an export in each
// non-UTF-8 encoding decodes back to the table it was given. A write-back over
// a file read with --encoding must keep the file's encoding, or the next read
// of it with the same flag returns mojibake.
func TestDumpTableWritesTheSourceEncoding(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.Header{"name"}, []model.Record{
		model.Record([]string{"日本語"}),
	})
	const want = "name\n日本語\n"

	for enc, decoder := range map[model.TextEncoding]encoding.Encoding{
		model.TextEncodingShiftJIS:  japanese.ShiftJIS,
		model.TextEncodingEUCJP:     japanese.EUCJP,
		model.TextEncodingISO2022JP: japanese.ISO2022JP,
		model.TextEncodingUTF16LE:   unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM),
		model.TextEncodingUTF16BE:   unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM),
	} {
		t.Run(string(enc), func(t *testing.T) {
			t.Parallel()

			out := filepath.Join(t.TempDir(), "out.csv")
			if err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionNone, enc); err != nil {
				t.Fatalf("DumpTable(%s) = %v", enc, err)
			}
			raw, err := os.ReadFile(out) //nolint:gosec // the test's own temp file
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "日本語") {
				t.Fatalf("DumpTable(%s) wrote UTF-8", enc)
			}
			got, err := decoder.NewDecoder().Bytes(raw)
			if err != nil {
				t.Fatalf("decode %s: %v", enc, err)
			}
			if string(got) != want {
				t.Errorf("DumpTable(%s) decodes to %q, want %q", enc, got, want)
			}
		})
	}
}

// TestDumpTableRefusesAValueTheEncodingCannotWrite checks that a rune the
// target encoding has no bytes for fails the export, naming the encoding,
// instead of being replaced in the user's file.
func TestDumpTableRefusesAValueTheEncodingCannotWrite(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.Header{"name"}, []model.Record{
		model.Record([]string{"🍣"}),
	})
	out := filepath.Join(t.TempDir(), "out.csv")
	err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionNone, model.TextEncodingShiftJIS)
	if err == nil {
		t.Fatal("DumpTable wrote a value Shift-JIS has no bytes for")
	}
	if !strings.Contains(err.Error(), string(model.TextEncodingShiftJIS)) {
		t.Errorf("error %q does not name the encoding", err)
	}
}
