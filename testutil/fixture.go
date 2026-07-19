package testutil

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// AssertFileEquals compares got with the fixture file at path after normalizing
// line endings, so snapshots stay stable across platforms.
func AssertFileEquals(t *testing.T, path string, got []byte) {
	t.Helper()

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	if diff := cmp.Diff(string(normalizeLF(want)), string(normalizeLF(got))); diff != "" {
		t.Fatalf("fixture mismatch for %s (-want +got):\n%s", path, diff)
	}
}

func normalizeLF(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})
	return bytes.ReplaceAll(data, []byte{'\r'}, []byte{'\n'})
}
