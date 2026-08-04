package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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
		"doc/build_and_test.md",
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

	shell, err := os.ReadFile("website/content/shell.md")
	if err != nil {
		t.Fatalf("read the shell page: %v", err)
	}
	dump := lineContaining(string(shell), "`.dump TABLE FILE`")
	if dump == "" {
		t.Fatal("the shell page no longer documents .dump")
	}
	if !strings.Contains(dump, "extension") {
		t.Errorf(".dump docs should describe extension-driven format inference in table mode, got: %s", dump)
	}

	reference, err := os.ReadFile("website/content/reference.md")
	if err != nil {
		t.Fatalf("read the reference page: %v", err)
	}
	if !strings.Contains(string(reference), "ACH") || !strings.Contains(string(reference), "Fedwire") {
		t.Errorf("the write-back reference should mention ACH/Fedwire whole-set write-back")
	}
}

// lineContaining returns the first line of doc holding needle, or "" when no
// line does. The helper-command reference is a table, so one row is the unit a
// docs-sync check asserts on.
func lineContaining(doc, needle string) string {
	for line := range strings.SplitSeq(doc, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// makefileTargets returns the set of target names declared in the Makefile (a
// line of the form "name:" at column 0). Pattern rules and variables are ignored.
func makefileTargets(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // test reads repository documentation paths
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
	data, err := os.ReadFile(path) //nolint:gosec // test reads repository documentation paths
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

// TestDemoAssets_TapesAndGIFsStayInStep guards the demo GIFs, within what a test
// can actually see.
//
// What it verifies: every tape declares exactly one Output GIF and that file
// exists; every GIF the README embeds is produced by a tape; no tape types a
// dot-command the shell no longer has; and no GIF was committed before the tape
// it is rendered from was last changed.
//
// What it cannot verify: the pixels. Rendering needs vhs, ttyd, and ffmpeg and is
// far too heavy for CI, and nothing here reads a GIF's frames. A GIF re-rendered
// from the right tape is the only thing that makes its contents current — the
// commit-order check below is what makes "someone edited a tape and forgot to run
// make demo" fail, rather than a claim that the frames were inspected.
//
// The flags a tape types are checked separately, by
// TestDemoTapes_EveryRecordedInvocationParses, which runs them through the real
// argument parser.
func TestDemoAssets_TapesAndGIFsStayInStep(t *testing.T) {
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
		assertGIFNotOlderThanTape(t, tape, gif)
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

// removedShellCommandsInTapes are dot-command names the shell no longer
// registers. A tape typing one renders a demo of sqly printing an error, and
// nothing else notices: the parser check only sees command-line arguments.
var removedShellCommandsInTapes = []string{".import-mode", ".ragged-rows", ".compare", ".profile", ".save --force"}

// TestDemoTapes_TypeNoRemovedShellCommand keeps the shell demos off the removed
// surface. The flags are covered by the parser check; this is the half of a tape
// that runs inside a session.
func TestDemoTapes_TypeNoRemovedShellCommand(t *testing.T) {
	t.Parallel()

	tapes, err := filepath.Glob("doc/vhs/*.tape")
	if err != nil {
		t.Fatalf("glob the tapes: %v", err)
	}
	for _, tape := range tapes {
		data, readErr := os.ReadFile(tape) //nolint:gosec // path comes from a glob over the repository's own tapes
		if readErr != nil {
			t.Fatalf("read %s: %v", tape, readErr)
		}
		for _, removed := range removedShellCommandsInTapes {
			if strings.Contains(string(data), removed) {
				t.Errorf("%s types %q, which the shell no longer has; the rendered GIF would show an error", tape, removed)
			}
		}
	}
}

// assertGIFNotOlderThanTape fails when a tape was committed more recently than
// the GIF it renders, which is what "edited the tape, forgot `make demo`" looks
// like in history. It uses commit timestamps rather than file mtimes because a
// checkout gives every file the same arbitrary mtime. When either path has no
// commit yet (both are staged in the same change, or this is a shallow clone),
// there is nothing to compare and the check passes.
func assertGIFNotOlderThanTape(t *testing.T, tape, gif string) {
	t.Helper()

	tapeTime, ok := lastCommitUnix(tape)
	if !ok {
		return
	}
	gifTime, ok := lastCommitUnix(gif)
	if !ok {
		return
	}
	if gifTime < tapeTime {
		t.Errorf("%s was last changed after %s was rendered; run `make demo` and commit the GIF", tape, gif)
	}
}

// lastCommitUnix returns the Unix time of the last commit touching path. The
// bool is false when git is unavailable, the repository has no history for the
// path, or the command fails for any other reason — none of which is something a
// docs test should fail on.
func lastCommitUnix(path string) (int64, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	//nolint:gosec // path comes from a glob over the repository's own doc and tape files
	out, err := exec.CommandContext(ctx, "git", "log", "-1", "--format=%ct", "--", path).Output()
	if err != nil {
		return 0, false
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, false
	}
	seconds, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, false
	}
	return seconds, true
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
	data, err := os.ReadFile(path) //nolint:gosec // path is a repo-relative doc file, not user input
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
