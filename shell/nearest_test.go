package shell

import (
	"math/rand"
	"strings"
	"testing"
)

// TestOSADistance pins the edit distance, including the swap that plain
// Levenshtein charges twice. A swap is the typo a "did you mean" most needs to
// catch, so charging it as one edit is what lets the threshold stay tight.
func TestOSADistance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a, b string
		want int
	}{
		{name: "identical strings are zero apart", a: "name", b: "name", want: 0},
		{name: "an empty string is as far as the other is long", a: "", b: "name", want: 4},
		{name: "both empty are zero apart", a: "", b: "", want: 0},
		{name: "one substitution is one edit", a: "nome", b: "name", want: 1},
		{name: "one deletion is one edit", a: "nae", b: "name", want: 1},
		{name: "one insertion is one edit", a: "namae", b: "name", want: 1},
		{name: "two adjacent runes swapped is one edit", a: "naem", b: "name", want: 1},
		{name: "a swap in a longer word is still one edit", a: "tabels", b: "tables", want: 1},
		{name: "two separate typos are two edits", a: "nmea", b: "name", want: 2},
		{name: "nothing in common costs the longer length", a: "xyz", b: "name", want: 4},
		{name: "distance is symmetric", a: "name", b: "naem", want: 1},
		{name: "multi-byte runes count as one each", a: "日本語", b: "日本", want: 1},
		{name: "swapped multi-byte runes are one edit", a: "本日語", b: "日本語", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := osaDistance(tt.a, tt.b); got != tt.want {
				t.Errorf("osaDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestOSADistanceProperties checks the invariants a distance must hold over
// random pairs: it is symmetric, zero only for equal strings, never longer than
// the longer string, and never shorter than their difference in length. A
// distance that breaks one of these would make "did you mean" offer nonsense.
func TestOSADistanceProperties(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(20260831)) //nolint:gosec // reproducible test input, not security
	alphabet := []rune("abc日")

	for i := range 3000 {
		a := randomWord(rng, alphabet, 6)
		b := randomWord(rng, alphabet, 6)

		d := osaDistance(a, b)
		switch {
		case d != osaDistance(b, a):
			t.Fatalf("case %d: osaDistance(%q, %q) is not symmetric", i, a, b)
		case (d == 0) != (a == b):
			t.Fatalf("case %d: osaDistance(%q, %q) = %d, but the strings are %s", i, a, b, d, equalityWord(a == b))
		case d > max(len([]rune(a)), len([]rune(b))):
			t.Fatalf("case %d: osaDistance(%q, %q) = %d, longer than the longer string", i, a, b, d)
		case d < abs(len([]rune(a))-len([]rune(b))):
			t.Fatalf("case %d: osaDistance(%q, %q) = %d, shorter than their difference in length", i, a, b, d)
		}
	}
}

func randomWord(rng *rand.Rand, alphabet []rune, maxLen int) string {
	var b strings.Builder
	for range rng.Intn(maxLen + 1) {
		b.WriteRune(alphabet[rng.Intn(len(alphabet))])
	}
	return b.String()
}

func equalityWord(equal bool) string {
	if equal {
		return "equal"
	}
	return "different"
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// TestNearestName covers what is offered and, as much, what is not: a guess at
// a word nothing resembles is worse than no guess, because it sends the reader
// after a name that was never the one they wanted.
func TestNearestName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		target     string
		candidates []string
		want       string
		wantOK     bool
	}{
		{
			name:       "a swapped pair of letters is offered",
			target:     "naem",
			candidates: []string{"identifier", "name", "age"},
			want:       "name",
			wantOK:     true,
		},
		{
			name:       "a dropped letter is offered",
			target:     "tabels",
			candidates: []string{"tables", "schema", "describe"},
			want:       "tables",
			wantOK:     true,
		},
		{
			name:       "the match ignores case",
			target:     "NAEM",
			candidates: []string{"name"},
			want:       "name",
			wantOK:     true,
		},
		{
			name:       "an exact name is its own nearest",
			target:     "name",
			candidates: []string{"name", "nome"},
			want:       "name",
			wantOK:     true,
		},
		{
			name:       "a word nothing resembles is not guessed at",
			target:     "zzzzzz",
			candidates: []string{"name", "age", "city"},
			wantOK:     false,
		},
		{
			name:       "a word of two runes is too short to guess at",
			target:     "id",
			candidates: []string{"is", "in"},
			wantOK:     false,
		},
		{
			name:       "a short word gets one edit, not two",
			target:     "namx",
			candidates: []string{"name"},
			want:       "name",
			wantOK:     true,
		},
		{
			name:       "a short word two edits away is not offered",
			target:     "nmxe",
			candidates: []string{"name"},
			wantOK:     false,
		},
		{
			name:       "no candidates means no guess",
			target:     "name",
			candidates: nil,
			wantOK:     false,
		},
		{
			name:       "the closer of two candidates wins",
			target:     "naem",
			candidates: []string{"nome", "name"},
			want:       "name",
			wantOK:     true,
		},
		{
			name:       "a tie goes to the candidate offered first",
			target:     "nam",
			candidates: []string{"name", "nams"},
			want:       "name",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := nearestName(tt.target, tt.candidates)
			if ok != tt.wantOK {
				t.Fatalf("nearestName(%q, %v) ok = %v, want %v (got %q)", tt.target, tt.candidates, ok, tt.wantOK, got)
			}
			if ok && got != tt.want {
				t.Errorf("nearestName(%q, %v) = %q, want %q", tt.target, tt.candidates, got, tt.want)
			}
		})
	}
}
