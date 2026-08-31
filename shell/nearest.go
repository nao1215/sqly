package shell

import "strings"

// nearestName returns the candidate closest to target, when one is close enough
// to be worth offering as "did you mean". It reports false when nothing is.
//
// A typo is usually one slip: a letter dropped, doubled, mistyped, or two
// letters swapped. The distance is therefore the optimal string alignment
// variant of Levenshtein, which counts a swap as one edit rather than two —
// ".tabels" for ".tables" and "naem" for "name" are both swaps, and both are
// two edits away under plain Levenshtein, which is far enough that the
// threshold would have to be loose enough to guess wildly at everything else.
//
// The comparison ignores case, because SQL identifiers do.
//
// Ties go to the first candidate, so the caller decides what wins by the order
// it passes them in: a column of the table the statement names should be
// offered before one that merely exists in the session.
func nearestName(target string, candidates []string) (string, bool) {
	limit := nearestLimit(len([]rune(target)))
	if limit == 0 {
		return "", false
	}

	best, bestDistance := "", limit+1
	for _, candidate := range candidates {
		d := osaDistance(strings.ToLower(target), strings.ToLower(candidate))
		if d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	if bestDistance > limit {
		return "", false
	}
	return best, true
}

// nearestLimit is how far a candidate may be from a word of n runes and still
// be offered. A short word turns into another short word in one edit — "id" is
// one edit from "is", "in", and "a" — so guessing at one says nothing, and the
// limit for those is no guess at all.
func nearestLimit(n int) int {
	switch {
	case n <= 2:
		return 0
	case n <= 4:
		return 1
	default:
		return 2
	}
}

// osaDistance is the optimal string alignment distance between a and b: the
// number of insertions, deletions, substitutions, and swaps of two adjacent
// runes needed to turn one into the other.
//
// It is the restricted edit distance, not the full Damerau-Levenshtein one: a
// substring is never edited twice, so "ca" to "abc" costs three here and two
// there. The difference needs two overlapping swaps to appear, which is not a
// typo anyone makes in one word, and the restricted form needs two rows of
// memory rather than a full matrix.
func osaDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}

	// prev2, prev and cur are the rows for i-2, i-1 and i of the matrix. Only
	// three are ever live: prev2 is what a swap looks back at.
	prev2 := make([]int, len(br)+1)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(
				prev[j]+1,      // delete
				cur[j-1]+1,     // insert
				prev[j-1]+cost, // substitute
			)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1) // swap two adjacent runes
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(br)]
}
