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
			if target, ok := firstMakeTarget(cmd); ok {
				refs = append(refs, docMakeRef{target: target, line: lineNo})
			}
			continue
		}
		for _, m := range inlineMake.FindAllStringSubmatch(line, -1) {
			if target, ok := firstMakeTarget("make " + m[1]); ok {
				refs = append(refs, docMakeRef{target: target, line: lineNo})
			}
		}
	}
	return refs, scanner.Err()
}

// firstMakeTarget reports the first non-flag argument of a "make ..." command,
// which names the target being invoked. It returns ok=false when the command is
// not a make invocation or carries no target (a bare "make").
func firstMakeTarget(cmd string) (string, bool) {
	fields := strings.Fields(cmd)
	if len(fields) < 2 || fields[0] != "make" {
		return "", false
	}
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") || strings.Contains(f, "=") {
			continue // skip flags and variable overrides
		}
		return f, true
	}
	return "", false
}
