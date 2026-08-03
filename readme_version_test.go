package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestREADMEVersionMatchesChangelog guards against invented version strings in
// the user-facing docs: every "sqly vX.Y.Z" reference in the README or on the
// website must be a version CHANGELOG.md actually records, so a typo or a
// version that was never released fails here.
//
// It covers the website as well as the README because the benchmark detail
// lives there now, and a version claim is exactly as wrong on a published page
// as in the README.
//
// It deliberately does not demand the latest one. The references today are
// benchmark captions naming the version the numbers were measured on — a
// release that did not re-run the benchmark must not rewrite them, or the
// caption claims a measurement nobody took. Re-measuring is what updates it, and
// the comparison table needs trdsql, csvq, and textql on one machine to stay
// consistent, so it is a deliberate act rather than a release chore.
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

	sources, err := filepath.Glob(filepath.Join("website", "content", "*.md"))
	if err != nil {
		t.Fatalf("glob website content: %v", err)
	}
	sources = append(sources, "README.md")

	// Match the version that trails an explicit "sqly v" reference, which is what a
	// release bump must keep current. Other incidental version strings are ignored.
	versionRe := regexp.MustCompile(`sqly (v\d+\.\d+\.\d+)`)
	found := 0
	for _, source := range sources {
		content, err := os.ReadFile(source) //nolint:gosec // paths come from a fixed glob in the repo
		if err != nil {
			t.Fatalf("read %s: %v", source, err)
		}
		for _, m := range versionRe.FindAllStringSubmatch(string(content), -1) {
			found++
			if !released[m[1]] {
				t.Errorf("%s has %q, which CHANGELOG.md does not record as a released version", source, m[0])
			}
		}
	}
	if found == 0 {
		t.Fatal(`no "sqly vX.Y.Z" reference found in the README or the website content`)
	}
}
