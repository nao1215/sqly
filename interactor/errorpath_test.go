package interactor

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nao1215/sqly/domain/model"
)

// TestDumpTable_InitCompressionError covers the withCompressedWriter branch where
// building the compression codec fails: bzip2 has no writer, so DumpTable must
// surface an "init compression" error after the output file is created.
func TestDumpTable_InitCompressionError(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("t", model.NewHeader([]string{"id"}), []model.Record{
		model.Record([]string{"1"}),
	})
	out := filepath.Join(t.TempDir(), "out.csv.bz2")

	err := exp.DumpTable(out, table, model.ExportCSV, model.CompressionBzip2)
	if err == nil {
		t.Fatal("DumpTable with bzip2 compression = nil error, want error (bzip2 has no writer)")
	}
}

// TestDumpTable_Parquet covers the ExportParquet case of DumpTable by exporting a
// non-empty table and asserting the file is written.
func TestDumpTable_Parquet(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("people", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.Record([]string{"1", "alice"}),
	})
	out := filepath.Join(t.TempDir(), "people.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err != nil {
		t.Fatalf("DumpTable Parquet failed: %v", err)
	}
}

// TestDumpTable_ParquetEmpty covers the ExportParquet error path routed through
// DumpTable: an empty result cannot be written as Parquet.
func TestDumpTable_ParquetEmpty(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	table := model.NewTable("empty", model.NewHeader([]string{"id"}), []model.Record{})
	out := filepath.Join(t.TempDir(), "empty.parquet")

	if err := exp.DumpTable(out, table, model.ExportParquet, model.CompressionNone); err == nil {
		t.Fatal("DumpTable Parquet on empty result = nil error, want error")
	}
}

// TestDumpTable_ReportsTheSerializerFailure checks that a value the format
// cannot carry is reported rather than written. What happens to the user's
// destination when it is not this scratch path is decided a layer up, by the
// staging and rename in shell.writeFileAtomically and in the save flow, which is
// where the preservation tests live.
func TestDumpTable_ReportsTheSerializerFailure(t *testing.T) {
	t.Parallel()

	exp := newTestExportInteractor()
	// A tab in a value has no representation in LTSV, so the dump must fail.
	table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.Record([]string{"1", "alice\tbob"}),
	})
	staging := filepath.Join(t.TempDir(), "staging.ltsv")

	if err := exp.DumpTable(staging, table, model.ExportLTSV, model.CompressionNone); err == nil {
		t.Fatal("DumpTable with a tab in an LTSV value = nil error, want error")
	}
}

// TestDumpTable_WritesOnlyToThePathItIsGiven pins that the export leaves nothing
// behind anywhere else: it used to serialize into a file in the OS temp
// directory and copy that across, so a failure could strand one there.
func TestDumpTable_WritesOnlyToThePathItIsGiven(t *testing.T) {
	exp := newTestExportInteractor()
	table := model.NewTable("t", model.NewHeader([]string{"id"}), []model.Record{
		model.Record([]string{"1"}),
	})

	before, err := filepath.Glob(filepath.Join(os.TempDir(), "sqly-export-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := exp.DumpTable(filepath.Join(dir, "ok.csv"), table, model.ExportCSV, model.CompressionNone); err != nil {
		t.Fatalf("DumpTable = %v, want nil", err)
	}
	// A serializer failure is the case that used to leave the strays.
	bad := model.NewTable("t", model.NewHeader([]string{"id"}), []model.Record{
		model.Record([]string{"a\tb"}),
	})
	if err := exp.DumpTable(filepath.Join(dir, "bad.ltsv"), bad, model.ExportLTSV, model.CompressionNone); err == nil {
		t.Fatal("DumpTable with a tab in an LTSV value = nil error, want error")
	}

	after, err := filepath.Glob(filepath.Join(os.TempDir(), "sqly-export-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("export left %d file(s) in the OS temp directory, want none", len(after)-len(before))
	}
}

// TestDumpTable_RoundTripsThroughItsOwnOutput checks that what the export writes
// is what the format reads back, for a compressed and an uncompressed
// destination. Writing the bytes once instead of copying them between two files
// must not change the file that results.
func TestDumpTable_RoundTripsThroughItsOwnOutput(t *testing.T) {
	t.Parallel()

	table := model.NewTable("t", model.NewHeader([]string{"id", "name"}), []model.Record{
		model.Record([]string{"1", "alice"}),
		model.Record([]string{"2", "bob"}),
	})
	const want = "id,name\n1,alice\n2,bob\n"

	for _, tt := range []struct {
		name string
		file string
		comp model.Compression
		read func(t *testing.T, path string) string
	}{
		{
			name: "uncompressed csv is the bytes themselves",
			file: "out.csv",
			comp: model.CompressionNone,
			read: func(t *testing.T, path string) string {
				t.Helper()
				b, err := os.ReadFile(path) //nolint:gosec // test path under t.TempDir
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			},
		},
		{
			name: "gzip csv decompresses to the same bytes",
			file: "out.csv.gz",
			comp: model.CompressionGzip,
			read: func(t *testing.T, path string) string {
				t.Helper()
				f, err := os.Open(path) //nolint:gosec // test path under t.TempDir
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = f.Close() }()
				zr, err := gzip.NewReader(f)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = zr.Close() }()
				b, err := io.ReadAll(zr)
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := filepath.Join(t.TempDir(), tt.file)
			if err := newTestExportInteractor().DumpTable(out, table, model.ExportCSV, tt.comp); err != nil {
				t.Fatalf("DumpTable = %v, want nil", err)
			}
			if got := tt.read(t, out); got != want {
				t.Errorf("round trip = %q, want %q", got, want)
			}
		})
	}
}

func TestDumpTable_PreservesFilePermissions(t *testing.T) {
	t.Parallel()

	t.Run("preserves file permissions on successful overwrite", func(t *testing.T) {
		t.Parallel()

		exp := newTestExportInteractor()
		table := model.NewTable("test", model.NewHeader([]string{"id", "name"}), []model.Record{
			model.Record([]string{"1", "alice"}),
		})

		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "output.ltsv")

		// Create file with custom permission (0o600)
		const customPerm = os.FileMode(0o600)
		if err := os.WriteFile(outPath, []byte("old content"), customPerm); err != nil {
			t.Fatalf("failed to write original file: %v", err)
		}

		infoBefore, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file before: %v", err)
		}

		// Perform successful dump
		if err := exp.DumpTable(outPath, table, model.ExportLTSV, model.CompressionNone); err != nil {
			t.Fatalf("failed to dump table: %v", err)
		}

		infoAfter, err := os.Stat(outPath)
		if err != nil {
			t.Fatalf("failed to stat file after: %v", err)
		}

		if infoBefore.Mode().Perm() != infoAfter.Mode().Perm() {
			t.Errorf("file permissions changed! got %v, want %v", infoAfter.Mode().Perm(), infoBefore.Mode().Perm())
		}
	})
}
