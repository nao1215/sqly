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

// TestSnapshotToCache_LoadFromCache_RoundTrip snapshots a loaded database to a
// standalone cache file, then reloads it into a fresh database and asserts the
// rows survive the round-trip.
func TestSnapshotToCache_LoadFromCache_RoundTrip(t *testing.T) {
	t.Parallel()

	src := covFsqlNewAdapter(t)
	ctx := context.Background()
	csv := covFsqlWriteCSV(t, "cachedata.csv", "id,name\n1,alice\n2,bob\n3,carol\n")
	if err := src.LoadFile(ctx, csv); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	cachePath := filepath.Join(t.TempDir(), "cache.db")
	// Pre-create the cache file so the stale-cache removal branch runs.
	if err := os.WriteFile(cachePath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("seed stale cache: %v", err)
	}
	if err := src.SnapshotToCache(ctx, cachePath); err != nil {
		t.Fatalf("SnapshotToCache: %v", err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache not written: %v", err)
	}

	dst := covFsqlNewAdapter(t)
	if err := dst.LoadFromCache(ctx, cachePath); err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}

	got, err := dst.Query(ctx, "SELECT name FROM cachedata ORDER BY id")
	if err != nil {
		t.Fatalf("query reloaded: %v", err)
	}
	names := make([]string, 0, len(got.Records()))
	for _, r := range got.Records() {
		names = append(names, r[0])
	}
	want := []string{"alice", "bob", "carol"}
	if len(names) != len(want) {
		t.Fatalf("reloaded names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("reloaded names = %v, want %v", names, want)
		}
	}
}

// TestSnapshotToCache_NilDB checks the guard when the shared database is nil.
func TestSnapshotToCache_NilDB(t *testing.T) {
	t.Parallel()

	a := NewFileSQLAdapter(nil)
	if err := a.SnapshotToCache(context.Background(), filepath.Join(t.TempDir(), "c.db")); err == nil {
		t.Fatal("SnapshotToCache with nil DB = nil error, want error")
	}
}

// TestLoadFromCache_NilDB checks the guard when the shared database is nil.
func TestLoadFromCache_NilDB(t *testing.T) {
	t.Parallel()

	a := NewFileSQLAdapter(nil)
	if err := a.LoadFromCache(context.Background(), filepath.Join(t.TempDir(), "c.db")); err == nil {
		t.Fatal("LoadFromCache with nil DB = nil error, want error")
	}
}

// TestLoadFromCache_Missing checks that loading from a nonexistent cache path
// returns an unavailable error rather than attaching a bogus database.
func TestLoadFromCache_Missing(t *testing.T) {
	t.Parallel()

	a := covFsqlNewAdapter(t)
	err := a.LoadFromCache(context.Background(), filepath.Join(t.TempDir(), "nope.db"))
	if err == nil {
		t.Fatal("LoadFromCache on missing path = nil error, want error")
	}
	if !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("error = %q, want it to mention the cache is unavailable", err.Error())
	}
}

// TestLoadFromCache_NoTables checks that a cache holding no user tables reports a
// clear error instead of silently succeeding. An empty database is snapshotted
// to produce such a cache.
func TestLoadFromCache_NoTables(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	empty := covFsqlNewAdapter(t)
	cachePath := filepath.Join(t.TempDir(), "emptycache.db")
	if err := empty.SnapshotToCache(ctx, cachePath); err != nil {
		t.Fatalf("SnapshotToCache(empty): %v", err)
	}

	dst := covFsqlNewAdapter(t)
	err := dst.LoadFromCache(ctx, cachePath)
	if err == nil {
		t.Fatal("LoadFromCache on empty cache = nil error, want error")
	}
	if !strings.Contains(err.Error(), "no tables") {
		t.Errorf("error = %q, want it to mention there are no tables", err.Error())
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

// TestSheetNames_Success reads the worksheet list from a real .xlsx workbook.
func TestSheetNames_Success(t *testing.T) {
	t.Parallel()

	xlsx := filepath.Join("..", "..", "testdata", "sample.xlsx")
	if _, err := os.Stat(xlsx); os.IsNotExist(err) {
		t.Skip("xlsx test data not available")
	}
	names, err := SheetNames(xlsx)
	if err != nil {
		t.Fatalf("SheetNames: %v", err)
	}
	if len(names) == 0 {
		t.Error("SheetNames returned no sheets, want at least one")
	}
}

// TestSheetNames_MissingFile checks the error path when the workbook cannot be
// opened.
func TestSheetNames_MissingFile(t *testing.T) {
	t.Parallel()

	if _, err := SheetNames(filepath.Join(t.TempDir(), "nope.xlsx")); err == nil {
		t.Fatal("SheetNames on missing file = nil error, want error")
	}
}

// TestSheetNames_NotExcel checks the error path when the file exists but is not a
// valid Excel workbook.
func TestSheetNames_NotExcel(t *testing.T) {
	t.Parallel()

	path := covFsqlWriteCSV(t, "not-excel.xlsx", "id,name\n1,alice\n")
	if _, err := SheetNames(path); err == nil {
		t.Fatal("SheetNames on non-Excel content = nil error, want error")
	}
}

// TestReadDecompressed_Plain reads an uncompressed file verbatim.
func TestReadDecompressed_Plain(t *testing.T) {
	t.Parallel()

	path := covFsqlWriteCSV(t, "plain.json", "[]")
	data, err := readDecompressed(path)
	if err != nil {
		t.Fatalf("readDecompressed: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("content = %q, want %q", string(data), "[]")
	}
}

// TestReadDecompressed_Missing checks the error path when the file is absent.
func TestReadDecompressed_Missing(t *testing.T) {
	t.Parallel()

	if _, err := readDecompressed(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("readDecompressed on missing file = nil error, want error")
	}
}

// TestEmptyJSONLikeTable_ReadError checks that a JSON/JSONL path that cannot be
// read is reported as not-empty (so the caller lets filesql surface the real
// error) rather than being misdetected as an empty table.
func TestEmptyJSONLikeTable_ReadError(t *testing.T) {
	t.Parallel()

	for _, ext := range []string{".json", ".jsonl"} {
		name, isEmpty := emptyJSONLikeTable(filepath.Join(t.TempDir(), "missing"+ext))
		if isEmpty || name != "" {
			t.Errorf("emptyJSONLikeTable(missing%s) = (%q, %v), want (\"\", false)", ext, name, isEmpty)
		}
	}
}

// TestEmptyJSONLikeTable_NonEmpty checks that a populated JSON array and a
// populated JSONL file are not treated as empty tables.
func TestEmptyJSONLikeTable_NonEmpty(t *testing.T) {
	t.Parallel()

	jsonPath := covFsqlWriteCSV(t, "full.json", `[{"a":1}]`)
	if _, isEmpty := emptyJSONLikeTable(jsonPath); isEmpty {
		t.Error("populated JSON array detected as empty")
	}
	jsonlPath := covFsqlWriteCSV(t, "full.jsonl", "{\"a\":1}\n")
	if _, isEmpty := emptyJSONLikeTable(jsonlPath); isEmpty {
		t.Error("populated JSONL detected as empty")
	}
	// A non-JSON extension is never an empty-JSON table.
	if _, isEmpty := emptyJSONLikeTable(covFsqlWriteCSV(t, "data.csv", "id\n1\n")); isEmpty {
		t.Error("csv detected as empty JSON table")
	}
}

// TestCreateEmptyJSONTable_Error checks the error branch: a closed database makes
// the CREATE/DROP statements fail.
func TestCreateEmptyJSONTable_Error(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	a := NewFileSQLAdapter(db)
	_ = db.Close() // force subsequent statements to fail

	if err := a.createEmptyJSONTable(context.Background(), "t"); err == nil {
		t.Fatal("createEmptyJSONTable on closed DB = nil error, want error")
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
