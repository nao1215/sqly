package filesql

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// writeBenchCSV generates a CSV with the given number of data rows. The columns
// deliberately mix the types the JSON contract distinguishes — an integer, a
// real, a zero-padded code that must stay text, and a plain text column — so the
// benchmark exercises the representation under measurement rather than a single
// uniform column.
func writeBenchCSV(b *testing.B, path string, rows int) {
	b.Helper()

	f, err := os.Create(path) //nolint:gosec // benchmark writes to a temp path it owns
	if err != nil {
		b.Fatal(err)
	}
	w := newBufWriter(f)
	if _, err := w.WriteString("id,amount,code,name\n"); err != nil {
		b.Fatal(err)
	}
	for i := range rows {
		line := strconv.Itoa(i) + "," +
			strconv.FormatFloat(float64(i)+0.5, 'f', 2, 64) + "," +
			fmt.Sprintf("%08d", i) + "," +
			"name-" + strconv.Itoa(i) + "\n"
		if _, err := w.WriteString(line); err != nil {
			b.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
}

// newBufWriter keeps the fixture writer dependency-free.
func newBufWriter(w io.Writer) *bufWriter { return &bufWriter{w: w, buf: make([]byte, 0, 1<<20)} }

type bufWriter struct {
	w   io.Writer
	buf []byte
}

func (b *bufWriter) WriteString(s string) (int, error) {
	b.buf = append(b.buf, s...)
	if len(b.buf) >= 1<<20 {
		if err := b.Flush(); err != nil {
			return 0, err
		}
	}
	return len(s), nil
}

func (b *bufWriter) Flush() error {
	if len(b.buf) == 0 {
		return nil
	}
	_, err := b.w.Write(b.buf)
	b.buf = b.buf[:0]
	return err
}

// benchImport measures a cold import of a single CSV of the given size.
func benchImport(b *testing.B, rows int) {
	b.Helper()

	dir := b.TempDir()
	csv := filepath.Join(dir, "bench.csv")
	writeBenchCSV(b, csv, rows)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			b.Fatal(err)
		}
		adapter := NewFileSQLAdapter(db)
		b.StartTimer()

		if err := adapter.LoadFiles(ctx, csv); err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		_ = db.Close()
		b.StartTimer()
	}
}

// BenchmarkImportCSV100k measures the 100,000-row import.
func BenchmarkImportCSV100k(b *testing.B) { benchImport(b, 100_000) }

// BenchmarkImportCSV1M measures the 1,000,000-row import.
func BenchmarkImportCSV1M(b *testing.B) { benchImport(b, 1_000_000) }

// BenchmarkImportMultiFile measures an ordered multi-file import, which is the
// path that runs inside one transaction.
func BenchmarkImportMultiFile(b *testing.B) {
	dir := b.TempDir()
	paths := make([]string, 0, 8)
	for i := range 8 {
		p := filepath.Join(dir, "part"+strconv.Itoa(i)+".csv")
		writeBenchCSV(b, p, 25_000)
		paths = append(paths, p)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			b.Fatal(err)
		}
		adapter := NewFileSQLAdapter(db)
		b.StartTimer()

		if err := adapter.LoadFiles(ctx, paths...); err != nil {
			b.Fatal(err)
		}

		b.StopTimer()
		_ = db.Close()
		b.StartTimer()
	}
}

// benchQueryFixture imports a CSV once and returns an adapter over it, so the
// query and output benchmarks measure only the work they name.
func benchQueryFixture(b *testing.B, rows int) (*FileSQLAdapter, func()) {
	b.Helper()

	dir := b.TempDir()
	csv := filepath.Join(dir, "bench.csv")
	writeBenchCSV(b, csv, rows)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatal(err)
	}
	adapter := NewFileSQLAdapter(db)
	if err := adapter.LoadFiles(context.Background(), csv); err != nil {
		b.Fatal(err)
	}
	return adapter, func() { _ = db.Close() }
}

// BenchmarkQueryMaterialize100k measures turning a 100,000-row result set into a
// *model.Table. This is where the per-cell representation is built, so it is the
// benchmark that would show a regression from keeping native values.
func BenchmarkQueryMaterialize100k(b *testing.B) {
	adapter, cleanup := benchQueryFixture(b, 100_000)
	defer cleanup()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		table, err := adapter.Query(ctx, "SELECT * FROM bench")
		if err != nil {
			b.Fatal(err)
		}
		if len(table.Records()) != 100_000 {
			b.Fatalf("rows = %d", len(table.Records()))
		}
	}
}

// benchPrint measures one output format over a materialized 100,000-row result.
func benchPrint(b *testing.B, mode model.PrintMode) {
	b.Helper()

	adapter, cleanup := benchQueryFixture(b, 100_000)
	defer cleanup()

	table, err := adapter.Query(context.Background(), "SELECT * FROM bench")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := table.Print(io.Discard, mode); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPrintCSV measures CSV output of a 100,000-row result.
func BenchmarkPrintCSV(b *testing.B) { benchPrint(b, model.PrintModeCSV) }

// BenchmarkPrintJSON measures JSON output of a 100,000-row result.
func BenchmarkPrintJSON(b *testing.B) { benchPrint(b, model.PrintModeJSON) }

// BenchmarkPrintNDJSON measures NDJSON output of a 100,000-row result.
func BenchmarkPrintNDJSON(b *testing.B) { benchPrint(b, model.PrintModeNDJSON) }

// BenchmarkPrintTable measures table output. It uses a smaller result because
// the table renderer measures every cell's display width, which is quadratic in
// neither rows nor columns but is far more expensive per cell than the
// delimited writers.
func BenchmarkPrintTable(b *testing.B) {
	adapter, cleanup := benchQueryFixture(b, 10_000)
	defer cleanup()

	table, err := adapter.Query(context.Background(), "SELECT * FROM bench")
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := table.Print(io.Discard, model.PrintModeTable); err != nil {
			b.Fatal(err)
		}
	}
}
