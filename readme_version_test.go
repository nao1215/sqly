package main

import (
	"os"
	"regexp"
	"testing"
)

// TestREADMEVersionMatchesChangelog guards against release-era version drift in
// the README: every "sqly vX.Y.Z" string (the shell welcome snippet and the
// benchmark caption) must match the latest version heading in CHANGELOG.md. When a
// release bumps the changelog, this fails until the README is refreshed too, so
// stale version strings cannot silently linger. Ref #454.
func TestREADMEVersionMatchesChangelog(t *testing.T) {
	t.Parallel()

	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	// The first "## [vX.Y.Z]" heading is the latest released version.
	headingRe := regexp.MustCompile(`(?m)^## \[(v\d+\.\d+\.\d+)\]`)
	heading := headingRe.FindSubmatch(changelog)
	if heading == nil {
		t.Fatal("no version heading found in CHANGELOG.md")
	}
	latest := string(heading[1])

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
		if m[1] != latest {
			t.Errorf("README.md has %q but the latest CHANGELOG version is %q; refresh the README version strings", m[0], latest)
		}
	}
}
