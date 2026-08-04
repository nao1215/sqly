package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/nao1215/sqly/testutil"
	"github.com/xuri/excelize/v2"
)

// The atago specs need corrupt inputs as real files, and a corrupt file cannot
// testify to being corrupt. A fixture named broken.xlsx that quietly became a
// readable workbook would make every rollback scenario using it pass while
// testing the successful path instead — the scenario would still see an import
// succeed and a table appear, and would report that as the contract holding.
//
// So the two committed fixtures are written by the same generator the Go tests
// use (testutil.WriteCorruptFixture, which verifies its own output), and read
// back here on every ordinary test run to check they are still broken in the
// way their name claims.

// corruptFixtures are the committed corrupt inputs and how each is damaged.
var corruptFixtures = []struct {
	path string
	kind testutil.CorruptKind
}{
	{
		// A real workbook with its tail cut away, so the ZIP central directory
		// is gone. The leading bytes are a valid ZIP header, which is the point:
		// a reader that only sniffs the first few bytes would accept it.
		path: filepath.Join("atago", "testdata", "corrupt_truncated.xlsx"),
		kind: testutil.CorruptTruncatedZip,
	},
	{
		// A zstd stream that stops early, so the failure is in the codec rather
		// than in what the codec was wrapping.
		path: filepath.Join("atago", "testdata", "corrupt_truncated.csv.zst"),
		kind: testutil.CorruptOuterZstd,
	},
}

// TestCorruptFixtures_AreStillCorrupt is the check that keeps the atago
// rollback scenarios meaningful.
func TestCorruptFixtures_AreStillCorrupt(t *testing.T) {
	t.Parallel()

	for _, fixture := range corruptFixtures {
		t.Run(fixture.path, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(fixture.path)
			if err != nil {
				t.Fatalf("read %s: %v (run `REGEN_CORRUPT_FIXTURES=1 go test ./e2e/ -run TestCorruptFixtures_Regenerate`)", fixture.path, err)
			}
			if len(data) == 0 {
				t.Fatalf("%s is empty; an empty file is a different failure from a damaged one", fixture.path)
			}

			switch fixture.kind {
			case testutil.CorruptTruncatedZip:
				if _, err := excelize.OpenReader(bytes.NewReader(data)); err == nil {
					t.Errorf("%s opens as a workbook; it is supposed to be truncated", fixture.path)
				}
				// And it really is a truncated archive rather than junk: the ZIP
				// local-file-header magic is still at the front.
				if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
					t.Errorf("%s does not start with a ZIP header; it is no longer the truncated-archive case", fixture.path)
				}

			case testutil.CorruptOuterZstd:
				zr, err := zstd.NewReader(bytes.NewReader(data))
				if err == nil {
					_, readErr := zr.DecodeAll(data, nil)
					zr.Close()
					if readErr == nil {
						t.Errorf("%s decompresses; its codec is supposed to be truncated", fixture.path)
					}
				}

			default:
				t.Fatalf("no verification written for %s", fixture.kind)
			}
		})
	}
}

// TestCorruptFixtures_Regenerate rewrites the committed fixtures. It is a test
// so it stays compiled beside the declaration it reads from, and does nothing
// unless asked.
//
//	REGEN_CORRUPT_FIXTURES=1 go test ./e2e/ -run TestCorruptFixtures_Regenerate
func TestCorruptFixtures_Regenerate(t *testing.T) {
	if os.Getenv("REGEN_CORRUPT_FIXTURES") == "" {
		t.Skip("set REGEN_CORRUPT_FIXTURES=1 to rewrite the corrupt fixtures")
	}
	dir := t.TempDir()
	for _, fixture := range corruptFixtures {
		// The generator names the file itself, from the kind's extension, so the
		// produced file is copied to the committed path.
		produced := testutil.WriteCorruptFixture(t, dir, "corrupt", fixture.kind)
		data, err := os.ReadFile(produced) //nolint:gosec // a path the generator just wrote
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Clean(fixture.path), data, 0o600); err != nil { //nolint:gosec // a fixed in-repo fixture path
			t.Fatalf("write %s: %v", fixture.path, err)
		}
		t.Logf("wrote %s (%s, %d bytes)", fixture.path, fixture.kind, len(data))
	}
}
