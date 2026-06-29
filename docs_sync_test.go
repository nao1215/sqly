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

// TestDemoAssetsInSync is a docs-sync guardrail for the demo GIFs. Rendering GIFs
// needs vhs/ttyd/ffmpeg (via `make demo`) and is too heavy for CI, so this does
// not render them; it checks that the assets a tape declares and the README
// references actually exist and line up. That catches the common drift the
// project cannot otherwise see: a doc/vhs/*.tape added or changed without its GIF
// regenerated, or a README GIF pointing at an asset no tape produces.
func TestDemoAssetsInSync(t *testing.T) {
	t.Parallel()

	tapeGIF, err := tapeOutputGIFs("doc/vhs")
	if err != nil {
		t.Fatalf("scan tapes: %v", err)
	}
	if len(tapeGIF) == 0 {
		t.Fatal("no tape Output directives parsed; the parser or the tapes changed")
	}

	// Every tape must declare exactly one Output GIF, and that GIF must exist. A
	// tape that was added or whose command changed without `make demo` being rerun
	// fails here, naming the missing asset.
	produced := map[string]bool{}
	for tape, gif := range tapeGIF {
		if gif == "" {
			t.Errorf("%s has no Output directive; a tape must declare the GIF it renders", tape)
			continue
		}
		produced[gif] = true
		if _, statErr := os.Stat(gif); statErr != nil {
			t.Errorf("%s declares Output %q, but that GIF is missing; run `make demo` to render it", tape, gif)
		}
	}

	// Every GIF the README embeds must exist and be produced by a tape, so a
	// documented demo cannot point at an asset nothing regenerates.
	refs, err := markdownGIFRefs("README.md")
	if err != nil {
		t.Fatalf("scan README.md: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("no doc/img GIF references found in README.md; the parser or the README changed")
	}
	for _, ref := range refs {
		if _, statErr := os.Stat(ref.path); statErr != nil {
			t.Errorf("README.md:%d references %q, which does not exist", ref.line, ref.path)
		}
		if !produced[ref.path] {
			t.Errorf("README.md:%d references %q, but no doc/vhs/*.tape produces it; add a tape or fix the reference", ref.line, ref.path)
		}
	}
}

// tapeOutputGIFs maps each doc/vhs/*.tape to the GIF path in its `Output "..."`
// directive (empty when a tape declares none). The path is repo-relative, matching
// how the README references it.
func tapeOutputGIFs(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	outputLine := regexp.MustCompile(`^Output\s+"([^"]+)"`)
	result := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".tape") {
			continue
		}
		path := dir + "/" + e.Name()
		data, readErr := os.ReadFile(path) //nolint:gosec // path is a repo-relative tape file
		if readErr != nil {
			return nil, readErr
		}
		result[path] = "" // record the tape even if it declares no Output
		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			if m := outputLine.FindStringSubmatch(strings.TrimSpace(scanner.Text())); m != nil {
				result[path] = m[1]
				break
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			return nil, scanErr
		}
	}
	return result, nil
}

// docImageRef is a Markdown image reference and the line it appears on.
type docImageRef struct {
	path string
	line int
}

// markdownGIFRefs returns the repo-relative doc/img/*.gif paths embedded in a
// Markdown file via the image syntax ![alt](path). A leading "./" is trimmed so
// the path matches a tape's Output directive.
func markdownGIFRefs(path string) ([]docImageRef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	image := regexp.MustCompile(`!\[[^\]]*\]\((\.?/?doc/img/[^)]+\.gif)\)`)
	var refs []docImageRef
	lineNo := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		lineNo++
		for _, m := range image.FindAllStringSubmatch(scanner.Text(), -1) {
			refs = append(refs, docImageRef{path: strings.TrimPrefix(m[1], "./"), line: lineNo})
		}
	}
	return refs, scanner.Err()
}
