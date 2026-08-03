package filesql

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	_ "modernc.org/sqlite"
)

// covFsqlNewAdapter returns an adapter bound to a fresh shared in-memory
// database. The pool is pinned to a single connection because a bare
// ":memory:" database is private per connection, so every statement must run
// against the same underlying database.
func covFsqlNewAdapter(t *testing.T) *FileSQLAdapter {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return NewFileSQLAdapter(db)
}

// covFsqlWriteCSV writes a small CSV file into a temp dir and returns its path.
func covFsqlWriteCSV(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	return path
}

// TestCompressionToLib_AllEnums checks every mappable Compression enum resolves
// to a filesql CompressionType without error, and that the two non-writable
// cases (bzip2 and an unknown value) return an error instead.
func TestCompressionToLib_AllEnums(t *testing.T) {
	t.Parallel()

	valid := []model.Compression{
		model.CompressionNone,
		model.CompressionGzip,
		model.CompressionXz,
		model.CompressionZstd,
		model.CompressionZlib,
		model.CompressionSnappy,
		model.CompressionS2,
		model.CompressionLz4,
	}
	for _, c := range valid {
		if _, err := compressionToLib(c); err != nil {
			t.Errorf("compressionToLib(%v) unexpected error: %v", c, err)
		}
	}

	if _, err := compressionToLib(model.CompressionBzip2); err == nil {
		t.Error("compressionToLib(bzip2) = nil error, want error (bzip2 has no writer)")
	}
	if _, err := compressionToLib(model.Compression(255)); err == nil {
		t.Error("compressionToLib(unknown) = nil error, want error")
	}
}

// TestNewCompressingWriter_None checks that CompressionNone returns the writer
// unchanged with a no-op close, so bytes pass through verbatim.
func TestNewCompressingWriter_None(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w, closeFn, err := NewCompressingWriter(&buf, model.CompressionNone)
	if err != nil {
		t.Fatalf("NewCompressingWriter(none): %v", err)
	}
	if _, err := io.WriteString(w, "hello world"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if buf.String() != "hello world" {
		t.Errorf("passthrough = %q, want %q", buf.String(), "hello world")
	}
}

// TestNewCompressingWriter_Gzip checks the gzip codec produces a stream that
// decompresses back to the original bytes.
func TestNewCompressingWriter_Gzip(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w, closeFn, err := NewCompressingWriter(&buf, model.CompressionGzip)
	if err != nil {
		t.Fatalf("NewCompressingWriter(gzip): %v", err)
	}
	const payload = "the quick brown fox"
	if _, err := io.WriteString(w, payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := closeFn(); err != nil {
		t.Fatalf("close: %v", err)
	}

	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer func() { _ = gr.Close() }()
	got, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("read decompressed: %v", err)
	}
	if string(got) != payload {
		t.Errorf("round-trip = %q, want %q", string(got), payload)
	}
}

// TestNewCompressingWriter_Error checks the error branch: an unwritable codec
// (bzip2) surfaces the mapping error and returns no writer.
func TestNewCompressingWriter_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w, closeFn, err := NewCompressingWriter(&buf, model.CompressionBzip2)
	if err == nil {
		t.Fatal("NewCompressingWriter(bzip2) = nil error, want error")
	}
	if w != nil || closeFn != nil {
		t.Error("on error, writer and close func should be nil")
	}
}

// TestCopyFile_RoundTrip copies a file and verifies the destination content
// matches the source exactly.
func TestCopyFile_RoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	want := []byte("payload-bytes-1234")
	if err := os.WriteFile(src, want, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dst) //nolint:gosec // dst is a controlled test temp path
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("copied content = %q, want %q", got, want)
	}
}

// TestCopyFile_ReadError checks copyFile surfaces the read error when the source
// does not exist.
func TestCopyFile_ReadError(t *testing.T) {
	t.Parallel()

	err := copyFile(filepath.Join(t.TempDir(), "missing.bin"), filepath.Join(t.TempDir(), "out.bin"))
	if err == nil {
		t.Fatal("copyFile with missing source = nil error, want error")
	}
}

// TestCopyFile_WriteError checks copyFile surfaces the write error when the
// destination directory does not exist.
func TestCopyFile_WriteError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}
	dst := filepath.Join(dir, "no-such-dir", "out.bin")
	if err := copyFile(src, dst); err == nil {
		t.Fatal("copyFile into missing dir = nil error, want error")
	}
}

// TestDumpTableToParquet_WriteError checks the export surfaces a clear error when
// the destination directory does not exist, exercising the copyFile write-error
// branch through the exporter.
func TestDumpTableToParquet_WriteError(t *testing.T) {
	t.Parallel()

	table := model.NewTable("people", model.Header{"id"}, []model.Record{{"1"}})
	dst := filepath.Join(t.TempDir(), "missing-dir", "people.parquet")
	err := DumpTableToParquet(dst, table)
	if err == nil {
		t.Fatal("DumpTableToParquet into missing dir = nil error, want error")
	}
	if !strings.Contains(err.Error(), "write parquet") {
		t.Errorf("error = %q, want it to mention writing the parquet file", err.Error())
	}
}

// TestDumpACHFile_RoundTrip loads an ACH file and dumps it back out, asserting a
// non-empty file is produced. It also checks the nil-DB guard.
//
// This test loads an ACH file, which registers a TableSet in filesql's
// process-global registry, so it must not run in parallel with other ACH tests.
func TestDumpACHFile_RoundTrip(t *testing.T) {
	achFile := filepath.Join("..", "..", "testdata", "ppd-debit.ach")
	if _, err := os.Stat(achFile); os.IsNotExist(err) {
		t.Skip("ACH test data not available")
	}

	ctx := context.Background()
	a := covFsqlNewAdapter(t)
	if err := a.LoadFile(ctx, achFile); err != nil {
		t.Fatalf("LoadFile ACH: %v", err)
	}

	baseName := GetTableNameFromFilePath(achFile)
	out := filepath.Join(t.TempDir(), "dumped.ach")
	if err := a.DumpACHFile(ctx, baseName, out); err != nil {
		t.Fatalf("DumpACHFile: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("dumped ACH not written: %v", err)
	}
	if info.Size() == 0 {
		t.Error("dumped ACH file is empty")
	}

	// nil-DB guard.
	if err := NewFileSQLAdapter(nil).DumpACHFile(ctx, baseName, out); err == nil {
		t.Error("DumpACHFile with nil DB = nil error, want error")
	}
}

// TestQuery_SQLError checks that an invalid query returns a FileSQLError.
func TestQuery_SQLError(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	if _, err := a.Query(context.Background(), "SELECT * FROM no_such_table"); err == nil {
		t.Fatal("Query on missing table = nil error, want error")
	}
}

// TestExec_SQLError checks that an invalid statement returns a FileSQLError.
func TestExec_SQLError(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	if _, err := a.Exec(context.Background(), "UPDATE no_such_table SET x = 1"); err == nil {
		t.Fatal("Exec on missing table = nil error, want error")
	}
}

// TestGetTableNames_ClosedDB checks the QueryContext error branch by closing the
// database before the call.
func TestGetTableNames_ClosedDB(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	a := NewFileSQLAdapter(db)
	_ = db.Close()

	if _, err := a.GetTableNames(context.Background()); err == nil {
		t.Fatal("GetTableNames on closed DB = nil error, want error")
	}
}

// TestGetTableHeader_EmptyName checks the empty-name guard.
func TestGetTableHeader_EmptyName(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	if _, err := a.GetTableHeader(context.Background(), "   "); err == nil {
		t.Fatal("GetTableHeader with blank name = nil error, want error")
	}
}

// TestGetTableHeader_ClosedDB checks the QueryContext error branch by closing the
// database before the call.
func TestGetTableHeader_ClosedDB(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	a := NewFileSQLAdapter(db)
	_ = db.Close()

	if _, err := a.GetTableHeader(context.Background(), "some_table"); err == nil {
		t.Fatal("GetTableHeader on closed DB = nil error, want error")
	}
}

// TestDumpFedWireFile_RoundTrip loads a Fedwire file and dumps it back out,
// asserting a non-empty file is produced. It also checks the nil-DB guard.
//
// This test loads a Fedwire file, which registers a TableSet in filesql's
// process-global registry, so it must not run in parallel with other FED tests.
func TestDumpFedWireFile_RoundTrip(t *testing.T) {
	fedFile := filepath.Join("..", "..", "testdata", "customer-transfer.fed")
	if _, err := os.Stat(fedFile); os.IsNotExist(err) {
		t.Skip("FED test data not available")
	}

	ctx := context.Background()
	a := covFsqlNewAdapter(t)
	if err := a.LoadFile(ctx, fedFile); err != nil {
		t.Fatalf("LoadFile FED: %v", err)
	}

	baseName := GetTableNameFromFilePath(fedFile)
	out := filepath.Join(t.TempDir(), "dumped.fed")
	if err := a.DumpFedWireFile(ctx, baseName, out); err != nil {
		t.Fatalf("DumpFedWireFile: %v", err)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("dumped FED not written: %v", err)
	}
	if info.Size() == 0 {
		t.Error("dumped FED file is empty")
	}

	// nil-DB guard.
	if err := NewFileSQLAdapter(nil).DumpFedWireFile(ctx, baseName, out); err == nil {
		t.Error("DumpFedWireFile with nil DB = nil error, want error")
	}
}
