package main

import (
	"os"
	"regexp"
	"testing"
)

// TestREADMEVersionMatchesChangelog guards against invented version strings in
// the README: every "sqly vX.Y.Z" reference must be a version CHANGELOG.md
// actually records, so a typo or a version that was never released fails here.
//
// It deliberately does not demand the latest one. The only such reference today
// is the benchmark caption, and that names the version the numbers were measured
// on — a release that did not re-run the benchmark must not rewrite it, or the
// caption claims a measurement nobody took. Re-measuring is what updates it, and
// the comparison table below it needs trdsql, csvq, and textql on one machine to
// stay consistent, so it is a deliberate act rather than a release chore.
func TestREADMEVersionMatchesChangelog(t *testing.T) {
	t.Parallel()

	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	headingRe := regexp.MustCompile(`(?m)^## \[(v\d+\.\d+\.\d+)\]`)
	headings := headingRe.FindAllSubmatch(changelog, -1)
	if headings == nil {
		t.Fatal("no version heading found in CHANGELOG.md")
	}
	released := make(map[string]bool, len(headings))
	for _, h := range headings {
		released[string(h[1])] = true
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	// Match the version that trails an explicit "sqly v" reference, which is what a
	// release bump must keep current. Other incidental version strings are ignored.
	versionRe := regexp.MustCompile(`sqly (v\d+\.\d+\.\d+)`)
	matches := versionRe.FindAllStringSubmatch(string(readme), -1)
	if len(matches) == 0 {
		t.Fatal(`no "sqly vX.Y.Z" reference found in README.md`)
	}
	for _, m := range matches {
		if !released[m[1]] {
			t.Errorf("README.md has %q, which CHANGELOG.md does not record as a released version", m[0])
		}
	}
}
