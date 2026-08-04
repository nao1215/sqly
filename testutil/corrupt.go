// Package testutil contains shared test fixtures and assertions.
package testutil

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/xuri/excelize/v2"
)

// A test that says "this file is broken" and proves nothing is worse than no
// test at all: the import it drives fails for whatever reason the file happens
// to have, and a fixture that quietly became readable would make every
// assertion about rollback pass while testing the successful path.
//
// So every corrupt fixture here is built from a known-good one and then damaged
// in a stated way, and every builder checks its own work: the good file opens,
// the damaged one does not, and the layer that is supposed to fail is the one
// that does. A generator that stopped corrupting would fail here rather than
// somewhere far away.

// CorruptKind names how a fixture is broken, and — just as importantly — which
// layer is expected to notice.
type CorruptKind int

const (
	// CorruptNotAZip is an .xlsx holding bytes that are not a ZIP archive at all.
	// The workbook reader rejects it immediately.
	CorruptNotAZip CorruptKind = iota
	// CorruptTruncatedZip is a real workbook with its tail cut off, taking the
	// ZIP central directory with it. The bytes start out valid, which is what
	// separates this from the case above: a reader that only sniffs the first
	// few bytes would accept it.
	CorruptTruncatedZip
	// CorruptInnerXLSX is a broken workbook inside an intact gzip stream.
	// Decompression succeeds and the parse then fails, so it distinguishes "the
	// codec was wrong" from "what was inside was wrong".
	CorruptInnerXLSX
	// CorruptOuterCompression is a valid file inside a gzip stream whose tail was
	// cut off. Decompression itself fails, which is the other half of that pair.
	CorruptOuterCompression
	// CorruptOuterZstd is CorruptOuterCompression for a second codec, so the
	// rollback contract is not accidentally a property of gzip.
	CorruptOuterZstd
	// CorruptTruncatedJSONL is a JSONL file cut off mid-document: a structured
	// text format rather than a container, so a rollback that only worked for
	// workbooks would show up.
	CorruptTruncatedJSONL
	// CorruptTrailingGarbageJSON is a valid JSON document with junk after it.
	CorruptTrailingGarbageJSON
	// CorruptInvalidParquet is a .parquet file whose magic footer is wrong.
	CorruptInvalidParquet
)

func (k CorruptKind) String() string {
	switch k {
	case CorruptNotAZip:
		return "not-a-zip xlsx"
	case CorruptTruncatedZip:
		return "truncated xlsx"
	case CorruptInnerXLSX:
		return "broken xlsx inside a good gzip"
	case CorruptOuterCompression:
		return "truncated gzip"
	case CorruptOuterZstd:
		return "truncated zstd"
	case CorruptTruncatedJSONL:
		return "truncated jsonl"
	case CorruptTrailingGarbageJSON:
		return "json with trailing garbage"
	case CorruptInvalidParquet:
		return "parquet with a bad footer"
	default:
		return "unknown"
	}
}

// Extension is the name a fixture of this kind must carry, because the format
// is chosen from it.
func (k CorruptKind) Extension() string {
	switch k {
	case CorruptInnerXLSX, CorruptOuterCompression:
		return ".xlsx.gz"
	case CorruptOuterZstd:
		return ".csv.zst"
	case CorruptTruncatedJSONL:
		return ".jsonl"
	case CorruptTrailingGarbageJSON:
		return ".json"
	case CorruptInvalidParquet:
		return ".parquet"
	default:
		return ".xlsx"
	}
}

// WriteCorruptFixture writes a corrupt file of the given kind at dir/name+ext
// and returns its path. It fails the test if the file it produced is not
// actually broken.
func WriteCorruptFixture(t *testing.T, dir, name string, kind CorruptKind) string {
	t.Helper()

	path := filepath.Join(dir, name+kind.Extension())
	if err := os.WriteFile(path, corruptBytes(t, kind), 0o600); err != nil {
		t.Fatalf("write the %s fixture: %v", kind, err)
	}
	verifyCorrupt(t, path, kind)
	return path
}

// corruptBytes builds the damaged content for a kind.
func corruptBytes(t *testing.T, kind CorruptKind) []byte {
	t.Helper()

	switch kind {
	case CorruptNotAZip:
		return []byte("this is not a zip archive, and never was\n")

	case CorruptTruncatedZip:
		return truncateTail(t, goodWorkbook(t), 3)

	case CorruptInnerXLSX:
		// A broken workbook, correctly gzipped. The codec is fine.
		return gzipBytes(t, truncateTail(t, goodWorkbook(t), 3))

	case CorruptOuterCompression:
		// A good workbook inside a gzip stream that stops early.
		return truncateTail(t, gzipBytes(t, goodWorkbook(t)), 4)

	case CorruptOuterZstd:
		return truncateTail(t, zstdBytes(t, []byte("v\n1\n")), 4)

	case CorruptTruncatedJSONL:
		return []byte("{\"id\":1,\"name\":\"a\"}\n{\"id\":2,\"na")

	case CorruptTrailingGarbageJSON:
		return []byte("[{\"id\":1}] not json any more {{{")

	case CorruptInvalidParquet:
		// Parquet is recognized by a "PAR1" magic at both ends. This has neither.
		return []byte("PAR0 definitely not a parquet file PAR0")

	default:
		t.Fatalf("unknown corrupt kind %d", int(kind))
		return nil
	}
}

// verifyCorrupt is the part that makes these fixtures worth having: it checks
// the file is broken, and broken in the way the kind claims.
func verifyCorrupt(t *testing.T, path string, kind CorruptKind) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // a path this helper just wrote
	if err != nil {
		t.Fatalf("read back the %s fixture: %v", kind, err)
	}

	switch kind {
	case CorruptNotAZip, CorruptTruncatedZip:
		if _, err := excelize.OpenReader(bytes.NewReader(data)); err == nil {
			t.Fatalf("the %s fixture at %s opens as a workbook; the generator stopped corrupting it", kind, path)
		}

	case CorruptInnerXLSX:
		// The gzip layer must be intact, and the workbook inside must not be.
		inner := gunzip(t, data)
		if inner == nil {
			t.Fatalf("the %s fixture at %s does not decompress; its outer codec was supposed to be valid", kind, path)
		}
		if _, err := excelize.OpenReader(bytes.NewReader(inner)); err == nil {
			t.Fatalf("the %s fixture at %s holds a readable workbook; the inner file was supposed to be broken", kind, path)
		}

	case CorruptOuterCompression:
		if gunzip(t, data) != nil {
			t.Fatalf("the %s fixture at %s decompresses; its outer codec was supposed to be truncated", kind, path)
		}

	case CorruptOuterZstd:
		if unzstd(t, data) != nil {
			t.Fatalf("the %s fixture at %s decompresses; its outer codec was supposed to be truncated", kind, path)
		}

	case CorruptTruncatedJSONL, CorruptTrailingGarbageJSON:
		if json.Valid(data) {
			t.Fatalf("the %s fixture at %s is valid JSON; the generator stopped corrupting it", kind, path)
		}

	case CorruptInvalidParquet:
		if len(data) >= 4 && bytes.Equal(data[:4], []byte("PAR1")) {
			t.Fatalf("the %s fixture at %s carries the parquet magic; it was supposed to be wrong", kind, path)
		}
	}
}

// goodWorkbook returns the bytes of a workbook that opens cleanly, and proves
// it before returning. Everything the workbook kinds damage starts here, so a
// generator that produced an unreadable "good" file would make the damaged ones
// meaningless.
func goodWorkbook(t *testing.T) []byte {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetCellValue("Sheet1", "A1", "v"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue("Sheet1", "A2", "1"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write a workbook: %v", err)
	}

	data := buf.Bytes()
	if _, err := excelize.OpenReader(bytes.NewReader(data)); err != nil {
		t.Fatalf("the undamaged workbook does not open: %v", err)
	}
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("the undamaged workbook is not a readable zip: %v", err)
	}
	return data
}

// truncateTail removes a fraction of the tail: 1/n of the bytes. For a ZIP that
// takes the central directory with it, which is at the end.
func truncateTail(t *testing.T, data []byte, n int) []byte {
	t.Helper()
	if len(data) < n*2 {
		t.Fatalf("cannot truncate %d bytes by a %dth", len(data), n)
	}
	return data[:len(data)-len(data)/n]
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zstdBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// gunzip returns the decompressed bytes, or nil when the stream does not
// decompress. It reads to the end on purpose: a truncated stream opens fine and
// fails only when the reader reaches the missing tail.
func gunzip(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil
	}
	return out
}

// unzstd is gunzip for zstd.
func unzstd(t *testing.T, data []byte) []byte {
	t.Helper()
	zr, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil
	}
	return out
}
