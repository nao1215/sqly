package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestDocsMakeCommandsExist is a docs-sync guardrail: every `make <target>`
// command shown in the contributor-facing docs must correspond to a real target
// in the Makefile. Without this, a stale instruction (for example `make install
// tools`, where `install` is not a target) can ship and waste a new
// contributor's time. The check only looks at command contexts (fenced code
// blocks and inline `make ...` code spans), so prose such as "make things" is
// ignored.
func TestDocsMakeCommandsExist(t *testing.T) {
	t.Parallel()

	targets, err := makefileTargets("Makefile")
	if err != nil {
		t.Fatalf("read Makefile targets: %v", err)
	}
	if len(targets) == 0 {
		t.Fatal("no targets parsed from Makefile; the parser or the file changed")
	}

	docs := []string{
		"README.md",
		"CONTRIBUTING.md",
		"doc/pages/markdown/build_and_test.md",
	}
	for _, doc := range docs {
		refs, err := docMakeTargets(doc)
		if err != nil {
			t.Fatalf("scan %s: %v", doc, err)
		}
		for _, ref := range refs {
			if !targets[ref.target] {
				t.Errorf("%s:%d documents `make %s`, but %q is not a Makefile target", doc, ref.line, ref.target, ref.target)
			}
		}
	}
}

// TestHelperCommandDocsMatchBehavior is a docs-sync guardrail for the
// helper-command reference: the .save and .dump descriptions must match the
// shell's current behavior. The .dump format in table mode is inferred from the
// destination extension (not always CSV), and .save can reconstruct a whole
// ACH/Fedwire set back to its native file. A stale description here misleads
// users about what these commands write.
func TestHelperCommandDocsMatchBehavior(t *testing.T) {
	t.Parallel()

	const path = "doc/pages/markdown/sqly_helper_command.md"
	dump, err := docSection(path, "dump command")
	if err != nil {
		t.Fatalf("read .dump section: %v", err)
	}
	if strings.Contains(dump, "If mode is table, .dump output CSV file") {
		t.Errorf(".dump docs still claim table mode always writes CSV; it infers the format from the destination extension")
	}
	if !strings.Contains(dump, "extension") {
		t.Errorf(".dump docs should describe extension-driven format inference in table mode")
	}

	save, err := docSection(path, "save command")
	if err != nil {
		t.Fatalf("read .save section: %v", err)
	}
	if !strings.Contains(save, "ACH") || !strings.Contains(save, "Fedwire") {
		t.Errorf(".save docs should mention ACH/Fedwire whole-set write-back")
	}
}

// docSection returns the body of the Markdown section introduced by the heading
// "### <title>", up to the next "### " heading or end of file. It lets a
// docs-sync test assert on one command's description without matching text from
// a neighboring section.
func docSection(path, title string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	heading := "### " + title
	var b strings.Builder
	inSection := false
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "### ") {
			if inSection {
				break // reached the next section
			}
			inSection = strings.TrimSpace(line) == heading
			continue
		}
		if inSection {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if !inSection && b.Len() == 0 {
		return "", os.ErrNotExist
	}
	return b.String(), scanner.Err()
}

// makefileTargets returns the set of target names declared in the Makefile (a
// line of the form "name:" at column 0). Pattern rules and variables are ignored.
func makefileTargets(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	targetLine := regexp.MustCompile(`^([a-zA-Z][\w-]*):`)
	targets := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		if m := targetLine.FindStringSubmatch(scanner.Text()); m != nil {
			targets[m[1]] = true
		}
	}
	return targets, scanner.Err()
}

// docMakeRef is a documented `make <target>` command and the line it appears on.
type docMakeRef struct {
	target string
	line   int
}

// docMakeTargets extracts the first non-flag argument of every `make` command in
// command contexts of a Markdown file: inside fenced code blocks (a line that,
// after an optional shell prompt, starts with "make ") and inline code spans
// (`make ...`). Prose mentions of "make" are not command contexts and are
// skipped, so a sentence like "make things easier" is never treated as a target.
func docMakeTargets(path string) ([]docMakeRef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	inlineMake := regexp.MustCompile("`make ([^`]+)`")
	var refs []docMakeRef
	inFence := false
	lineNo := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			cmd := strings.TrimSpace(line)
			cmd = strings.TrimPrefix(cmd, "$ ")
			cmd = strings.TrimSpace(cmd)
			for _, target := range makeCommandTargets(cmd) {
				refs = append(refs, docMakeRef{target: target, line: lineNo})
			}
			continue
		}
		for _, m := range inlineMake.FindAllStringSubmatch(line, -1) {
			for _, target := range makeCommandTargets("make " + m[1]) {
				refs = append(refs, docMakeRef{target: target, line: lineNo})
			}
		}
	}
	return refs, scanner.Err()
}

// makeValueFlags are the GNU make short options that consume the following token
// as their value (for example `make -C docs build`), so that token names a path
// or file, not a target.
var makeValueFlags = map[string]bool{"-C": true, "-f": true, "-I": true, "-o": true, "-W": true}

// makeCommandTargets returns every target named by a "make ..." command. It skips
// flags, the values of value-taking flags (-C/-f/-I/-o/-W), and variable
// overrides (NAME=value), so a command like `make -C docs build lint` yields
// ["build", "lint"]. A command that is not a make invocation, or a bare `make`,
// yields nothing.
func makeCommandTargets(cmd string) []string {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "make" {
		return nil
	}
	var targets []string
	for i := 1; i < len(fields); i++ {
		f := fields[i]
		if makeValueFlags[f] {
			i++ // this flag consumes the next token as its value
			continue
		}
		if strings.HasPrefix(f, "-") || strings.Contains(f, "=") {
			continue // other flags and variable overrides are not targets
		}
		targets = append(targets, f)
	}
	return targets
}
