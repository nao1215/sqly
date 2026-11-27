package shell

import "testing"

// TestRuneCursorAnswersOffsetsInAnyOrder checks the rune offset of each byte
// offset in a string holding multi-byte names, asked forwards, backwards and
// out of range.
func TestRuneCursorAnswersOffsetsInAnyOrder(t *testing.T) {
	t.Parallel()

	const s = "SELECT 名前 FROM 表"
	c := &runeCursor{s: s}
	for _, q := range []struct{ byteOffset, want int }{
		{7, 7}, {13, 9}, {len(s), 16}, {13, 9}, {0, 0}, {-1, 0}, {len(s) + 5, 16}, {7, 7},
	} {
		if got := c.at(q.byteOffset); got != q.want {
			t.Errorf("at(%d) = %d, want %d", q.byteOffset, got, q.want)
		}
	}
}

// TestStartsWithDigit checks the edges of the digit range and the empty string.
func TestStartsWithDigit(t *testing.T) {
	t.Parallel()

	for s, want := range map[string]bool{"0a": true, "9": true, "/1": false, ":1": false, "a1": false, "": false} {
		if got := startsWithDigit(s); got != want {
			t.Errorf("startsWithDigit(%q) = %v, want %v", s, got, want)
		}
	}
}
