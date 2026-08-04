// Package testutil contains shared test fixtures and assertions.
package testutil

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
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

// reporter is the part of *testing.T the builders below use. It is an interface
// so testutil's own tests can drive a generator into its failure branches and
// observe them, instead of failing the test that is checking the check.
type reporter interface {
	Helper()
	Fatal(args ...any)
	Fatalf(format string, args ...any)
}

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
	// CorruptInvalidParquet is a .parquet file that starts with the parquet
	// magic and ends with something else. Parquet is recognized by "PAR1" at
	// both ends, and everything that says what is in the file lives at the end,
	// so a good header over a bad footer is the case a reader that only sniffs
	// the first four bytes would accept.
	CorruptInvalidParquet
)

// parquetMagic bookends a real parquet file, at the front and at the back.
const parquetMagic = "PAR1"

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
		// Naming the number rather than saying "unknown" is what makes a failure
		// message from a kind nobody wrote a case for point at the kind.
		return fmt.Sprintf("unknown corrupt kind %d", int(k))
	}
}

// Extension is the name a fixture of this kind must carry, because the format
// is chosen from it. A kind nobody has written a case for gets "", so it cannot
// be written out under a plausible-looking name; WriteCorruptFixture refuses it.
func (k CorruptKind) Extension() string {
	switch k {
	case CorruptNotAZip, CorruptTruncatedZip:
		return ".xlsx"
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
		return ""
	}
}

// WriteCorruptFixture writes a corrupt file of the given kind at dir/name+ext
// and returns its path. It fails the test if the file it produced is not
// actually broken.
func WriteCorruptFixture(t *testing.T, dir, name string, kind CorruptKind) string {
	t.Helper()
	return writeCorruptFixture(t, dir, name, kind)
}

// writeCorruptFixture is WriteCorruptFixture's body, reporting through the
// narrower interface so testutil's own tests can watch a generator fail rather
// than take the failure.
func writeCorruptFixture(t reporter, dir, name string, kind CorruptKind) string {
	t.Helper()

	ext := kind.Extension()
	if ext == "" {
		t.Fatalf("no extension is defined for %s; a fixture of a kind nobody has written a case for would be named after a format it is not", kind)
		return ""
	}

	path := filepath.Join(dir, name+ext)
	if err := os.WriteFile(path, corruptBytes(t, kind), 0o600); err != nil {
		t.Fatalf("write the %s fixture: %v", kind, err)
	}
	verifyCorrupt(t, path, kind)
	return path
}

// corruptBytes builds the damaged content for a kind.
func corruptBytes(t reporter, kind CorruptKind) []byte {
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
		// A good header over a bad footer, which is what "bad footer" says. The
		// front magic is left intact on purpose: the footer is where a parquet
		// file keeps its schema and row groups, so breaking only the end is both
		// the realistic damage and the case a reader that sniffs the first four
		// bytes and stops would wave through.
		return []byte(parquetMagic + " definitely not a parquet file PAR0")

	default:
		t.Fatalf("unknown corrupt kind %d", int(kind))
		return nil
	}
}

// verifyCorrupt is the part that makes these fixtures worth having: it checks
// the file is broken, and broken in the way the kind claims.
func verifyCorrupt(t reporter, path string, kind CorruptKind) {
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

	case CorruptTruncatedJSONL:
		// json.Valid over the whole file proves nothing here: JSONL is one
		// document per line, so a perfectly good two-line file is invalid read as
		// a single document. The damage is specifically that the last line stops
		// mid-document while every line before it is complete, and that is what
		// has to be checked — otherwise a generator that started emitting valid
		// JSONL would still pass.
		lines := nonEmptyLines(data)
		if len(lines) < 2 {
			t.Fatalf("the %s fixture at %s has %d non-empty line(s); it needs a complete line before the truncated one, or it is not testing a partial read", kind, path, len(lines))
		}
		for i, line := range lines[:len(lines)-1] {
			if !json.Valid(line) {
				t.Fatalf("line %d of the %s fixture at %s is not valid JSON: %q; only the last line is supposed to be broken", i+1, kind, path, line)
			}
		}
		if last := lines[len(lines)-1]; json.Valid(last) {
			t.Fatalf("the last line of the %s fixture at %s is valid JSON: %q; the generator stopped truncating it", kind, path, last)
		}

	case CorruptTrailingGarbageJSON:
		if json.Valid(data) {
			t.Fatalf("the %s fixture at %s is valid JSON; the generator stopped corrupting it", kind, path)
		}

	case CorruptInvalidParquet:
		// Both ends, because both ends are the contract. A check of the front
		// alone cannot tell a bad footer from a good file, which is the one thing
		// this kind claims.
		if len(data) < 2*len(parquetMagic) {
			t.Fatalf("the %s fixture at %s is %d bytes, too short to carry a magic at each end", kind, path, len(data))
		}
		if head := data[:len(parquetMagic)]; !bytes.Equal(head, []byte(parquetMagic)) {
			t.Fatalf("the %s fixture at %s starts with %q, not %q; it is supposed to look like a parquet file until its footer is read", kind, path, head, parquetMagic)
		}
		if tail := data[len(data)-len(parquetMagic):]; bytes.Equal(tail, []byte(parquetMagic)) {
			t.Fatalf("the %s fixture at %s ends with %q; its footer was supposed to be wrong", kind, path, parquetMagic)
		}

	default:
		t.Fatalf("no verification is written for %s; a new kind needs a check here, or its fixture only claims to be broken", kind)
	}
}

// nonEmptyLines splits data into lines and drops the blank ones, so a trailing
// newline is not mistaken for a final, empty document.
func nonEmptyLines(data []byte) [][]byte {
	var lines [][]byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, bytes.TrimSpace(line))
		}
	}
	return lines
}

// goodWorkbook returns the bytes of a workbook that opens cleanly, and proves
// it before returning. Everything the workbook kinds damage starts here, so a
// generator that produced an unreadable "good" file would make the damaged ones
// meaningless.
func goodWorkbook(t reporter) []byte {
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
func truncateTail(t reporter, data []byte, n int) []byte {
	t.Helper()
	if len(data) < n*2 {
		t.Fatalf("cannot truncate %d bytes by a %dth", len(data), n)
	}
	return data[:len(data)-len(data)/n]
}

func gzipBytes(t reporter, data []byte) []byte {
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

func zstdBytes(t reporter, data []byte) []byte {
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
func gunzip(t reporter, data []byte) []byte {
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
func unzstd(t reporter, data []byte) []byte {
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
