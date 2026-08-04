package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The fixtures here exist to make a rollback test fail when nothing rolled
// back, and they can only do that if they are really broken. verifyCorrupt is
// what enforces that, so these tests check verifyCorrupt itself: each one hands
// it a fixture that has been repaired in one specific way and requires it to
// object. A check that passes for a file that is no longer damaged is the exact
// failure mode the whole file was written to prevent.

// TestWriteCorruptFixture_ProducesEveryKind is the baseline: the generator
// writes each kind, names it correctly, and its own verification accepts it.
func TestWriteCorruptFixture_ProducesEveryKind(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds() {
		t.Run(kind.String(), func(t *testing.T) {
			t.Parallel()

			path := WriteCorruptFixture(t, t.TempDir(), "fixture", kind)
			if !strings.HasSuffix(path, kind.Extension()) {
				t.Errorf("fixture %s does not end in %q; the format is chosen from the extension", path, kind.Extension())
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat the fixture: %v", err)
			}
			if info.Size() == 0 {
				t.Error("the fixture is empty; an empty file is a different failure from a damaged one")
			}
		})
	}
}

// TestVerifyCorrupt_JSONLKeepsEveryLineButTheLastValid pins the JSONL contract
// the way JSONL actually works: one document per line. json.Valid over the
// whole file would report a perfectly good two-line fixture as invalid, so it
// cannot tell a truncated tail from a healthy file.
func TestVerifyCorrupt_JSONLKeepsEveryLineButTheLastValid(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(WriteCorruptFixture(t, t.TempDir(), "fixture", CorruptTruncatedJSONL))
	if err != nil {
		t.Fatal(err)
	}

	lines := nonEmptyLines(data)
	if len(lines) < 2 {
		t.Fatalf("the fixture has %d non-empty line(s); a truncated tail needs a complete line in front of it", len(lines))
	}
	for i, line := range lines[:len(lines)-1] {
		if !json.Valid(line) {
			t.Errorf("line %d is not valid JSON: %q; every line but the last is supposed to be complete", i+1, line)
		}
	}
	if last := lines[len(lines)-1]; json.Valid(last) {
		t.Errorf("the last line %q is valid JSON; it is supposed to stop mid-document", last)
	}

	// And the whole file, as one document, is invalid — which is true of healthy
	// JSONL too, and is why the per-line check above is the one that means
	// something.
	if json.Valid(data) {
		t.Error("the whole fixture parses as one JSON document; it is not JSONL at all")
	}
}

// TestVerifyCorrupt_RejectsARepairedJSONLTail is the mutation the per-line
// check exists for: a generator that completes the final line. The old
// whole-file json.Valid check accepted exactly this.
func TestVerifyCorrupt_RejectsARepairedJSONLTail(t *testing.T) {
	t.Parallel()

	repaired := []byte("{\"id\":1,\"name\":\"a\"}\n{\"id\":2,\"name\":\"b\"}\n")
	if !json.Valid(nonEmptyLines(repaired)[1]) {
		t.Fatal("the repaired tail is not valid JSON, so this test would pass for the wrong reason")
	}
	assertVerifyRejects(t, repaired, CorruptTruncatedJSONL, "valid JSON")
}

// TestVerifyCorrupt_RejectsAnIncompleteLineBeforeTheLast keeps the other half
// honest. A fixture whose damage starts earlier is a different fixture, and it
// no longer isolates "the reader got part of the file".
func TestVerifyCorrupt_RejectsAnIncompleteLineBeforeTheLast(t *testing.T) {
	t.Parallel()

	assertVerifyRejects(t, []byte("{\"id\":1,\"na\n{\"id\":2,\"na"), CorruptTruncatedJSONL, "not valid JSON")
}

// TestVerifyCorrupt_RejectsASingleLineJSONL guards the "there was a good line
// first" half of the claim. One broken line proves nothing about a partial read.
func TestVerifyCorrupt_RejectsASingleLineJSONL(t *testing.T) {
	t.Parallel()

	assertVerifyRejects(t, []byte("{\"id\":2,\"na\n"), CorruptTruncatedJSONL, "non-empty line")
}

// TestVerifyCorrupt_ParquetKeepsItsHeaderAndBreaksItsFooter checks the fixture
// matches the name it carries. "bad footer" is a claim about the end of the
// file, and a check of the first four bytes cannot make it.
func TestVerifyCorrupt_ParquetKeepsItsHeaderAndBreaksItsFooter(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(WriteCorruptFixture(t, t.TempDir(), "fixture", CorruptInvalidParquet))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2*len(parquetMagic) {
		t.Fatalf("the fixture is %d bytes, too short to carry a magic at each end", len(data))
	}
	if head := string(data[:len(parquetMagic)]); head != parquetMagic {
		t.Errorf("the fixture starts with %q, want %q: it is supposed to look like parquet until the footer is read", head, parquetMagic)
	}
	if tail := string(data[len(data)-len(parquetMagic):]); tail == parquetMagic {
		t.Errorf("the fixture ends with %q; its footer is supposed to be wrong", parquetMagic)
	}
}

// TestVerifyCorrupt_RejectsARestoredParquetFooter is the mutation: put the
// magic back at the end and the fixture is no longer the case its name claims.
func TestVerifyCorrupt_RejectsARestoredParquetFooter(t *testing.T) {
	t.Parallel()

	assertVerifyRejects(t, []byte(parquetMagic+" body "+parquetMagic), CorruptInvalidParquet, "footer was supposed to be wrong")
}

// TestVerifyCorrupt_RejectsAParquetWithoutItsHeader is the other direction. A
// file that is not parquet from its first byte tests a cheaper rejection than
// the one this kind is for.
func TestVerifyCorrupt_RejectsAParquetWithoutItsHeader(t *testing.T) {
	t.Parallel()

	assertVerifyRejects(t, []byte("PAR0 body PAR0"), CorruptInvalidParquet, "supposed to look like a parquet file")
}

// TestVerifyCorrupt_RejectsATooShortParquet covers the length guard, without
// which the two magic comparisons would read out of range or overlap.
func TestVerifyCorrupt_RejectsATooShortParquet(t *testing.T) {
	t.Parallel()

	assertVerifyRejects(t, []byte("PAR1"), CorruptInvalidParquet, "too short")
}

// TestVerifyCorrupt_FailsOnAnUnknownKind is what keeps this file from going
// stale. Adding a CorruptKind and a generator for it, but no verification, must
// not quietly produce a fixture nobody checks.
func TestVerifyCorrupt_FailsOnAnUnknownKind(t *testing.T) {
	t.Parallel()

	unknown := unknownKind()
	path := filepath.Join(t.TempDir(), "fixture.bin")
	if err := os.WriteFile(path, []byte("anything"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{}
	runUntilFatal(func() { verifyCorrupt(rec, path, unknown) })
	if !rec.failed {
		t.Fatal("verifyCorrupt accepted a kind it has no case for; a fixture of that kind would only claim to be broken")
	}
	if !strings.Contains(rec.message, "no verification is written") {
		t.Errorf("message = %q, want it to say no verification is written for the kind", rec.message)
	}
}

// TestCorruptKind_UnknownValuesDoNotLookLikeAFormat covers the two methods a
// caller reaches before any verification runs. An unknown kind that reported an
// ".xlsx" extension would be written out under a name for a format it is not.
func TestCorruptKind_UnknownValuesDoNotLookLikeAFormat(t *testing.T) {
	t.Parallel()

	unknown := unknownKind()
	if ext := unknown.Extension(); ext != "" {
		t.Errorf("Extension() of an unknown kind = %q, want \"\": a fixture nobody wrote a case for must not be named after a format", ext)
	}
	if got := unknown.String(); !strings.Contains(got, strconv.Itoa(int(unknown))) {
		t.Errorf("String() of an unknown kind = %q, want it to name the value %d", got, int(unknown))
	}

	rec := &recorder{}
	runUntilFatal(func() { writeCorruptFixture(rec, t.TempDir(), "fixture", unknown) })
	if !rec.failed {
		t.Fatal("the generator wrote a fixture for a kind it has no case for")
	}
}

// TestCorruptKind_EveryKnownKindHasAnExtension is the companion: every kind
// that does have a generator must have a name to be written under.
func TestCorruptKind_EveryKnownKindHasAnExtension(t *testing.T) {
	t.Parallel()

	for _, kind := range allKinds() {
		if kind.Extension() == "" {
			t.Errorf("%s has no extension; WriteCorruptFixture would refuse to write it", kind)
		}
		if strings.HasPrefix(kind.String(), "unknown") {
			t.Errorf("kind %d has no name", int(kind))
		}
	}
}

// allKinds is every kind the generator knows how to build.
func allKinds() []CorruptKind {
	return []CorruptKind{
		CorruptNotAZip,
		CorruptTruncatedZip,
		CorruptInnerXLSX,
		CorruptOuterCompression,
		CorruptOuterZstd,
		CorruptTruncatedJSONL,
		CorruptTrailingGarbageJSON,
		CorruptInvalidParquet,
	}
}

// unknownKind is a value past the last declared kind, standing in for one a
// future change adds without a verification case.
func unknownKind() CorruptKind { return CorruptInvalidParquet + 1 }

// assertVerifyRejects writes data as a fixture of the given kind, bypassing the
// generator, and requires verifyCorrupt to object with a message mentioning
// want. This is how a "the damage was undone" mutation is checked without
// editing the generator.
func assertVerifyRejects(t *testing.T, data []byte, kind CorruptKind, want string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fixture"+kind.Extension())
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	rec := &recorder{}
	runUntilFatal(func() { verifyCorrupt(rec, path, kind) })
	if !rec.failed {
		t.Fatalf("verifyCorrupt accepted %q as a %s fixture; the damage it claims is not being checked", data, kind)
	}
	if !strings.Contains(rec.message, want) {
		t.Errorf("message = %q, want it to mention %q", rec.message, want)
	}
}

// recorder stands in for *testing.T so a failure can be observed instead of
// taken. Fatal and Fatalf panic with fatalSentinel, which is how they stop the
// function under test the way the real ones do.
type recorder struct {
	failed  bool
	message string
}

type fatalSentinel struct{}

func (r *recorder) Helper() {}

func (r *recorder) Fatal(args ...any) {
	r.failed = true
	r.message = fmt.Sprint(args...)
	panic(fatalSentinel{})
}

func (r *recorder) Fatalf(format string, args ...any) {
	r.failed = true
	r.message = fmt.Sprintf(format, args...)
	panic(fatalSentinel{})
}

// runUntilFatal runs f, absorbing the panic a recorder's Fatal raises and
// letting every other panic through.
func runUntilFatal(f func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalSentinel); !ok {
				panic(r)
			}
		}
	}()
	f()
}

// Compile-time proof the recorder can stand where *testing.T stands.
var _ reporter = (*recorder)(nil)
