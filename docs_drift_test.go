package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/shell"
)

const (
	// referencePage is the page that quotes most of sqly's messages verbatim.
	referencePage = "website/content/reference.md"
)

// docSources returns every file that shows a reader how to run sqly: the README,
// the cookbook, and every page of the site. A command or a helper name that
// appears in one of these is a promise, so the guards below read the whole set.
// The site pages are globbed rather than listed, so a page added later is
// checked without anyone remembering to register it.
func docSources(t *testing.T) []string {
	t.Helper()

	pages, err := filepath.Glob("website/content/*.md")
	if err != nil {
		t.Fatalf("glob the site content: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("no site pages found under website/content")
	}
	docs := make([]string, 0, 2+len(pages))
	docs = append(docs, "README.md", "doc/cookbook.md")
	return append(docs, pages...)
}

// TestDocs_EveryDocumentedInvocationParses runs every `sqly ...` command shown in
// the documentation through the real argument parser. Only a handful of the
// documented commands are executed end to end (e2e/atago/cookbook.atago.yaml),
// so without this a flag that is renamed or dropped would keep being advertised
// by the other ~80: the cookbook alone shows more commands than any suite will
// ever run. Parsing is the part that can be checked exhaustively and it catches
// the drift that actually happens — a flag that no longer exists, a short form
// that changed letter, a value flag documented as a boolean.
func TestDocs_EveryDocumentedInvocationParses(t *testing.T) {
	t.Parallel()

	total := 0
	for _, doc := range docSources(t) {
		for _, cmd := range shellCommandsIn(t, doc) {
			args, ok := sqlyInvocation(cmd.text)
			if !ok {
				continue
			}
			total++
			if _, err := config.NewArg(append([]string{"sqly"}, args...)); err != nil {
				t.Errorf("%s:%d documents a command the parser rejects: %s\n  %v", doc, cmd.line, cmd.text, err)
			}
		}
	}
	if total < 50 {
		t.Fatalf("only %d sqly invocations found across the docs; the extractor or the docs changed", total)
	}
}

// TestDemoTapes_EveryRecordedInvocationParses is the same guard for the VHS
// tapes. A tape is what the README's GIFs are rendered from, so a renamed flag
// left in one produces a demo of sqly printing an error — and nothing fails
// until someone runs `make demo` and watches it. The tapes are not Markdown, so
// the Markdown extractor above does not see them.
func TestDemoTapes_EveryRecordedInvocationParses(t *testing.T) {
	t.Parallel()

	tapes, err := filepath.Glob("doc/vhs/*.tape")
	if err != nil {
		t.Fatalf("glob the tapes: %v", err)
	}
	if len(tapes) == 0 {
		t.Fatal("no tapes found under doc/vhs")
	}

	total := 0
	for _, tape := range tapes {
		for _, cmd := range tapeCommands(t, tape) {
			args, ok := sqlyInvocation(cmd.text)
			if !ok {
				continue
			}
			total++
			if _, err := config.NewArg(append([]string{"sqly"}, args...)); err != nil {
				t.Errorf("%s:%d records a command the parser rejects: %s\n  %v", tape, cmd.line, cmd.text, err)
			}
		}
	}
	if total < 5 {
		t.Fatalf("only %d sqly invocations found across the tapes; the extractor or the tapes changed", total)
	}
}

// tapeCommands returns the shell commands a VHS tape types. Only `Type "..."`
// directives carry them; the rest of a tape is timing and terminal setup.
func tapeCommands(t *testing.T, path string) []docCommand {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path comes from a glob over the repository's own tapes
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	typed := regexp.MustCompile(`^Type\s+"(.*)"\s*$`)
	var cmds []docCommand
	for i, raw := range strings.Split(string(data), "\n") {
		m := typed.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil {
			continue
		}
		cmds = append(cmds, docCommand{text: m[1], line: i + 1})
	}
	return cmds
}

// TestDocs_EveryDocumentedShellCommandExists checks the helper commands the
// shell page lists against the ones the shell actually registers. A dot-command
// that is renamed or removed would otherwise stay documented, and the reference
// table is the only place most of them are written down.
func TestDocs_EveryDocumentedShellCommandExists(t *testing.T) {
	t.Parallel()

	registered := shell.NewCommands()
	documented := documentedShellCommands(t, "website/content/shell.md")
	if len(documented) < 10 {
		t.Fatalf("only %d dot-commands found on the shell page; the extractor or the page changed", len(documented))
	}
	for name, line := range documented {
		if _, ok := registered[name]; !ok {
			t.Errorf("website/content/shell.md:%d documents %s, which the shell does not register", line, name)
		}
	}
	// The other direction: a command the shell offers but no page mentions is a
	// feature nobody can discover.
	for name := range registered {
		if _, ok := documented[name]; !ok {
			t.Errorf("the shell registers %s, but website/content/shell.md does not document it", name)
		}
	}
}

// cookbookCoverage maps every task section of doc/cookbook.md to the atago spec
// that runs its commands against the real binary. Parsing (above) proves a
// documented command is still a valid invocation; only a spec proves it still
// does what the section says. Adding a section without deciding where it is
// exercised fails TestCookbook_EverySectionIsExercised, so the decision cannot
// be skipped.
var cookbookCoverage = map[string]string{
	"First look at a file":                 "e2e/atago/cookbook.atago.yaml",
	"Multiple files are one import":        "e2e/atago/multi_input.atago.yaml",
	"Convert between formats":              "e2e/atago/cookbook.atago.yaml",
	"Join across files":                    "e2e/atago/join.atago.yaml",
	"Join across formats":                  "e2e/atago/cookbook.atago.yaml",
	"Compressed files":                     "e2e/atago/compression_roundtrip.atago.yaml",
	"JSON and JSONL":                       "e2e/atago/cookbook.atago.yaml",
	"Excel workbooks":                      "e2e/atago/excel_sheets.atago.yaml",
	"Files over HTTP":                      "e2e/atago/http_import.atago.yaml",
	"Pipe data in":                         "e2e/atago/cookbook.atago.yaml",
	"Pipe data out":                        "e2e/atago/pipelines.atago.yaml",
	"Load a directory":                     "e2e/atago/cookbook.atago.yaml",
	"Run SQL or a sqly script from a file": "e2e/atago/sql_file.atago.yaml",
	"Analytics":                            "e2e/atago/cookbook.atago.yaml",
	"Write changes back":                   "e2e/atago/cookbook.atago.yaml",
	"Other SQL dialects":                   "e2e/atago/cookbook.atago.yaml",
	"Row mismatches":                       "e2e/atago/cookbook.atago.yaml",
	"Text encodings":                       "e2e/atago/encoding.atago.yaml",
	"Financial formats":                    "e2e/atago/ach_fedwire_writeback.atago.yaml",
	"Scripting":                            "e2e/atago/cookbook.atago.yaml",
}

// TestCookbook_EverySectionIsExercised keeps doc/cookbook.md and the E2E suite in
// lockstep in both directions: every task section names the spec that runs it,
// and every named spec exists.
func TestCookbook_EverySectionIsExercised(t *testing.T) {
	t.Parallel()

	// The lead section is the by-task index, not a recipe, so it has no spec.
	const indexSection = "Find a recipe by task"

	sections := markdownSections(t, "doc/cookbook.md")
	if len(sections) < 15 {
		t.Fatalf("only %d sections found in doc/cookbook.md; the parser or the file changed", len(sections))
	}
	for _, section := range sections {
		if section == indexSection {
			continue
		}
		spec, ok := cookbookCoverage[section]
		if !ok {
			t.Errorf("doc/cookbook.md adds the section %q, but cookbookCoverage does not say which atago spec runs it", section)
			continue
		}
		if _, err := os.Stat(spec); err != nil {
			t.Errorf("cookbookCoverage maps %q to %s, which does not exist", section, spec)
		}
	}

	known := map[string]bool{indexSection: true}
	for _, s := range sections {
		known[s] = true
	}
	for section := range cookbookCoverage {
		if !known[section] {
			t.Errorf("cookbookCoverage lists %q, which is no longer a section of doc/cookbook.md", section)
		}
	}
}

// TestSite_InternalLinksResolve checks that every site-internal link in the
// website content points at a page that exists, and every image at a committed
// file. Hugo does not fail a build on a dead relative link, so a page renamed
// without its inbound links updated would publish broken.
func TestSite_InternalLinksResolve(t *testing.T) {
	t.Parallel()

	pages := map[string]bool{"/": true}
	entries, err := os.ReadDir("website/content")
	if err != nil {
		t.Fatalf("read website/content: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == "_index.md" {
			continue
		}
		pages["/"+strings.TrimSuffix(name, ".md")+"/"] = true
	}
	// Generated by content/_content.gotmpl from doc/cookbook.md.
	pages["/cookbook/"] = true

	link := regexp.MustCompile(`\]\((/[^)]*)\)`)
	for _, doc := range docSources(t) {
		if !strings.HasPrefix(doc, "website/content/") {
			continue
		}
		data, readErr := os.ReadFile(doc) //nolint:gosec // doc is a repo-relative path from the repository's own doc set
		if readErr != nil {
			t.Fatalf("read %s: %v", doc, readErr)
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			for _, m := range link.FindAllStringSubmatch(line, -1) {
				target := m[1]
				if strings.HasPrefix(target, "/img/") {
					asset := filepath.Join("doc", filepath.Clean(strings.TrimPrefix(target, "/")))
					if _, statErr := os.Stat(asset); statErr != nil {
						t.Errorf("%s:%d links to %s, which is not a file under doc/img", doc, lineNo+1, target)
					}
					continue
				}
				page := target
				if i := strings.IndexByte(page, '#'); i >= 0 {
					page = page[:i]
				}
				if !pages[page] {
					t.Errorf("%s:%d links to %s, which is not a page of the site", doc, lineNo+1, target)
				}
			}
		}
	}
}

// docCommand is one command line found in a shell code block.
type docCommand struct {
	text string
	line int
}

// shellCommandsIn returns the command lines inside ```shell fences of a Markdown
// file, with a leading prompt stripped and backslash continuations joined. Other
// fence languages (text, sql, yaml) hold output and data, not commands.
func shellCommandsIn(t *testing.T, path string) []docCommand {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path is a repo-relative doc from a fixed list
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var cmds []docCommand
	inShell := false
	var pending strings.Builder
	pendingLine := 0
	for i, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inShell = strings.TrimPrefix(line, "```") == "shell"
			pending.Reset()
			continue
		}
		if !inShell || line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "$ ")
		if pending.Len() == 0 {
			pendingLine = i + 1
		}
		if cont, ok := strings.CutSuffix(line, `\`); ok {
			pending.WriteString(strings.TrimSpace(cont))
			pending.WriteByte(' ')
			continue
		}
		pending.WriteString(line)
		cmds = append(cmds, docCommand{text: pending.String(), line: pendingLine})
		pending.Reset()
	}
	return cmds
}

// sqlyInvocation returns the arguments of the sqly command in a documented
// command line, and whether the line runs sqly at all. It accepts the plain
// form, the `go run github.com/nao1215/sqly@latest` form the front pages use,
// and a pipeline whose sqly stage is not the first, so a recipe is checked
// however it is written. A command whose sqly stage pipes into something else
// keeps only the sqly stage.
func sqlyInvocation(cmd string) ([]string, bool) {
	for _, stage := range splitPipeline(cmd) {
		fields, err := shellFields(stage)
		if err != nil || len(fields) == 0 {
			continue
		}
		switch {
		case fields[0] == "sqly":
			return fields[1:], true
		case fields[0] == "go" && len(fields) > 2 && fields[1] == "run" && strings.HasPrefix(fields[2], "github.com/nao1215/sqly"):
			return fields[3:], true
		}
	}
	return nil, false
}

// splitPipeline splits a command line on unquoted pipes.
func splitPipeline(cmd string) []string {
	var stages []string
	var cur strings.Builder
	var quote byte
	for i := range len(cmd) {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == quote && (i == 0 || cmd[i-1] != '\\') {
				quote = 0
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			quote = c
			cur.WriteByte(c)
		case c == '|':
			stages = append(stages, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	stages = append(stages, cur.String())
	return stages
}

// shellFields splits a command into arguments the way a shell would: single and
// double quotes group, and a backslash escapes the next character outside single
// quotes. It is deliberately small — documented commands use quoting to hold SQL
// together, nothing more.
func shellFields(s string) ([]string, error) {
	var (
		fields []string
		cur    strings.Builder
		quote  byte
		filled bool
	)
	flush := func() {
		if filled {
			fields = append(fields, cur.String())
			cur.Reset()
			filled = false
		}
	}
	// Not a range-over-int loop: the escape case advances i to consume the
	// character it just wrote, and a range loop resets i every iteration.
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && quote != '\'' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			filled = true
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			cur.WriteByte(c)
			filled = true
		case c == '\'' || c == '"':
			quote = c
			filled = true
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteByte(c)
			filled = true
		}
	}
	flush()
	if quote != 0 {
		return nil, os.ErrInvalid
	}
	return fields, nil
}

// documentedShellCommands returns the dot-commands a page documents in its
// reference tables, mapped to the line they appear on. Only the leading code
// span of a table row counts, so a command mentioned in prose is not treated as
// a documented entry.
func documentedShellCommands(t *testing.T, path string) map[string]int {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path is a repo-relative doc
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	row := regexp.MustCompile("^\\| `(\\.[a-z-]+)")
	found := map[string]int{}
	for i, line := range strings.Split(string(data), "\n") {
		if m := row.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if _, seen := found[m[1]]; !seen {
				found[m[1]] = i + 1
			}
		}
	}
	return found
}

// markdownSections returns the "## " headings of a Markdown file, in order.
func markdownSections(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path is a repo-relative doc
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var sections []string
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			sections = append(sections, strings.TrimSpace(rest))
		}
	}
	return sections
}

// TestShellFields pins the tokenizer the doc guards rely on. It only has to
// handle the quoting documented commands actually use, but it has to handle it
// exactly: a command it mis-splits is a command the parser is asked the wrong
// question about, or skips entirely.
func TestShellFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"plain", `sqly --sql SELECT user.csv`, []string{"sqly", "--sql", "SELECT", "user.csv"}},
		{"double quotes hold spaces", `sqly --sql "SELECT * FROM user"`, []string{"sqly", "--sql", "SELECT * FROM user"}},
		{"single quotes hold spaces", `sqly --sql 'SELECT * FROM user'`, []string{"sqly", "--sql", "SELECT * FROM user"}},
		{"escaped quote inside double quotes", `sqly --sql "SELECT FROM \"user\""`, []string{"sqly", "--sql", `SELECT FROM "user"`}},
		{"backslash is literal inside single quotes", `sqly --sql 'a \$.name b'`, []string{"sqly", "--sql", `a \$.name b`}},
		{"escaped dollar outside quotes", `sqly --sql \$x`, []string{"sqly", "--sql", "$x"}},
		{"escaped space joins one field", `sqly my\ file.csv`, []string{"sqly", "my file.csv"}},
		{"empty quoted argument is kept", `sqly --output ""`, []string{"sqly", "--output", ""}},
		{"adjacent quoted parts are one field", `sqly "a"'b'`, []string{"sqly", "ab"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := shellFields(tt.input)
			if err != nil {
				t.Fatalf("shellFields(%q) error = %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("shellFields(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("shellFields(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}

	if _, err := shellFields(`sqly --sql "unterminated`); err == nil {
		t.Error("shellFields accepted an unterminated quote, want an error")
	}
}

// TestSqlyInvocation pins which documented command lines are recognized as sqly
// invocations, and with what arguments. A line wrongly skipped here is a command
// the guards never check.
func TestSqlyInvocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
		ok    bool
	}{
		{"plain", `sqly --output-format csv user.csv`, []string{"--output-format", "csv", "user.csv"}, true},
		{"go run form", `go run github.com/nao1215/sqly@latest --output-format json user.csv`, []string{"--output-format", "json", "user.csv"}, true},
		{"second stage of a pipeline", `cat user.csv | sqly --stdin-format csv --sql "SELECT 1"`, []string{"--stdin-format", "csv", "--sql", "SELECT 1"}, true},
		{"first stage of a pipeline", `sqly --output-format json user.csv | jq .`, []string{"--output-format", "json", "user.csv"}, true},
		{"a pipe inside quotes is not a stage", `sqly --sql "SELECT 'a|b'"`, []string{"--sql", "SELECT 'a|b'"}, true},
		{"not sqly", `brew install nao1215/tap/sqly`, nil, false},
		{"go install is not an invocation", `go install github.com/nao1215/sqly@latest`, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := sqlyInvocation(tt.input)
			if ok != tt.ok {
				t.Fatalf("sqlyInvocation(%q) ok = %v, want %v (args %#v)", tt.input, ok, tt.ok, got)
			}
			if !tt.ok {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("sqlyInvocation(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("sqlyInvocation(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestDialectsPage_PassThroughClaimsAreSpecified ties the dialect page's most
// perishable section to the spec that proves it. The "what passes through"
// examples are the ones that would quietly become wrong: each is a divergence
// sqly could fix later, and a page still listing a fixed divergence as a
// limitation is worse than no page. Every command quoted there must therefore
// appear in the spec named on the page, which asserts the output beside it.
func TestDialectsPage_PassThroughClaimsAreSpecified(t *testing.T) {
	t.Parallel()

	const (
		page    = "website/content/dialects.md"
		section = "## What passes through, and can differ"
		spec    = "e2e/atago/dialect_limits.atago.yaml"
	)

	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read %s: %v", page, err)
	}
	body := string(data)
	if !strings.Contains(body, spec) {
		t.Fatalf("%s no longer names %s as the spec behind its claims", page, spec)
	}

	start := strings.Index(body, section)
	if start < 0 {
		t.Fatalf("%s no longer has the %q section", page, section)
	}
	rest := body[start+len(section):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}

	specData, err := os.ReadFile(spec)
	if err != nil {
		t.Fatalf("read %s: %v", spec, err)
	}

	queries := regexp.MustCompile(`--sql "([^"]+)"`).FindAllStringSubmatch(rest, -1)
	if len(queries) < 3 {
		t.Fatalf("only %d quoted queries found in the pass-through section; the page or the parser changed", len(queries))
	}
	for _, m := range queries {
		if !strings.Contains(string(specData), m[1]) {
			t.Errorf("%s documents %q as a pass-through divergence, but %s does not assert it", page, m[1], spec)
		}
	}
}

// TestExitCodes_DocumentedTableMatchesTheConstants keeps the reference's exit
// code table and the codes sqly actually returns from drifting apart. The table
// is what a script author writes their `case $?` against, so a code that exists
// and is undocumented is unusable and a documented code that no longer exists is
// a wrong branch nobody notices until it is taken.
func TestExitCodes_DocumentedTableMatchesTheConstants(t *testing.T) {
	t.Parallel()

	const (
		page    = "website/content/reference.md"
		section = "## Exit codes"
	)

	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read %s: %v", page, err)
	}
	body := string(data)
	start := strings.Index(body, section)
	if start < 0 {
		t.Fatalf("%s no longer has the %q section", page, section)
	}
	rest := body[start+len(section):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		rest = rest[:end]
	}

	documented := make(map[string]bool)
	for _, m := range regexp.MustCompile("(?m)^\\| `(\\d+)` \\|").FindAllStringSubmatch(rest, -1) {
		documented[m[1]] = true
	}

	defined := map[string]string{
		strconv.Itoa(shell.ExitOK):         "ExitOK",
		strconv.Itoa(shell.ExitFailure):    "ExitFailure",
		strconv.Itoa(shell.ExitUsage):      "ExitUsage",
		strconv.Itoa(shell.ExitInput):      "ExitInput",
		strconv.Itoa(shell.ExitOutput):     "ExitOutput",
		strconv.Itoa(shell.ExitInterrupt):  "ExitInterrupt",
		strconv.Itoa(shell.ExitTerminated): "ExitTerminated",
	}

	for code, name := range defined {
		if !documented[code] {
			t.Errorf("shell.%s is %s, which %s does not document", name, code, page)
		}
	}
	for code := range documented {
		if _, ok := defined[code]; !ok {
			t.Errorf("%s documents exit code %s, which sqly no longer returns", page, code)
		}
	}
}

// TestFlags_EveryOneIsDocumentedInTheReference derives the flag list from the
// parser instead of trusting any document to keep its own copy. A flag that
// exists and is documented nowhere is one nobody can find; a flag documented
// after it has been removed sends a reader to write a command that fails.
//
// The reference is the page that promises to list every flag, so it is the one
// checked. The README deliberately does not enumerate them — it used to say
// "Twelve flags" while there were thirteen, which is exactly the drift a
// hand-maintained count produces.
func TestFlags_EveryOneIsDocumentedInTheReference(t *testing.T) {
	t.Parallel()

	const page = "website/content/reference.md"
	data, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read %s: %v", page, err)
	}
	body := string(data)

	// The names the page documents, as whole words. A prefix test would let
	// `--sql-file` stand in for `--sql`, so a flag could go undocumented and the
	// test would still pass.
	documented := make(map[string]bool)
	for _, m := range regexp.MustCompile("`--([a-z][a-z0-9-]*)").FindAllStringSubmatch(body, -1) {
		documented[m[1]] = true
	}

	flags := definedFlags(t)
	if len(flags) < 10 {
		t.Fatalf("only %d flags found; the extractor or the parser changed", len(flags))
	}
	for _, name := range flags {
		if !documented[name] {
			t.Errorf("--%s is a real flag that %s does not document", name, page)
		}
	}

	// And the reverse: a long flag the page mentions must still exist. The
	// exceptions are the options of dot-commands, which are the shell's language
	// rather than the CLI's and so are not in the FlagSet at all.
	dotCommandOptions := map[string]bool{
		"in-place":        true, // .save --in-place
		"follow-symlinks": true, // .save --in-place --follow-symlinks
	}
	defined := make(map[string]bool, len(flags))
	for _, name := range flags {
		defined[name] = true
	}
	for name := range documented {
		if !defined[name] && !dotCommandOptions[name] {
			t.Errorf("%s documents --%s, which the parser does not define", page, name)
		}
	}
}

// TestREADME_DoesNotHandCountTheFlags keeps the count from coming back. A number
// spelled out in prose has no way to notice a flag being added.
func TestREADME_DoesNotHandCountTheFlags(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	counted := regexp.MustCompile(`(?i)\b(one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|\d+)\s+flags\b`)
	if m := counted.FindString(string(data)); m != "" {
		t.Errorf("README.md states a flag count (%q); the number goes stale the next time a flag is added. Describe the groups instead.", m)
	}
}

// definedFlags returns every long flag name the parser defines, read from the
// usage the parser itself renders. Going through the real output is what makes
// this a check on sqly rather than on a second list kept beside it.
func definedFlags(t *testing.T) []string {
	t.Helper()

	arg, err := config.NewArg([]string{"sqly"})
	if err != nil {
		t.Fatalf("build the usage: %v", err)
	}
	seen := make(map[string]bool)
	var names []string
	for _, m := range regexp.MustCompile(`--([a-z][a-z0-9-]*)`).FindAllStringSubmatch(arg.Usage, -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		names = append(names, m[1])
	}
	return names
}

// flagToken matches a flag name as a whole token: followed by something that
// cannot continue a flag name, or by the end of the text. A plain substring
// search does not do this, and the difference is not academic — `--sql-file`
// contains `--sql`, so a README that had lost its `--sql` example once passed a
// drift test that only asked whether the string appeared.
func flagToken(flag string) *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta("--"+flag) + `($|[^a-z0-9-])`)
}

// mentionsFlag reports whether body uses the flag as its own token.
func mentionsFlag(body, flag string) bool {
	return flagToken(flag).MatchString(body)
}

// readDoc returns a documentation file's contents, failing the test if it is
// missing — a renamed page is drift too.
func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // fixed, in-repo documentation paths
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// flatten collapses every run of whitespace to a single space, so a claim that
// documentation wraps across two lines is still one string to search for. A
// check that breaks when a paragraph is rewrapped teaches contributors to
// reflow around the test rather than to keep the claim.
func flatten(body string) string {
	return strings.Join(strings.Fields(body), " ")
}

// section returns the body of a Markdown section by heading text, or "" when the
// heading is absent. Checking inside a section rather than across a whole file
// is what keeps a claim from being satisfied by an unrelated mention elsewhere.
//
// A section ends at the next heading of its own depth or shallower. Stopping
// only at its own depth would let a "###" section run on into the "##" that
// follows it, which is the case where the whole point of checking a section
// rather than a file is lost.
func section(body, heading string) string {
	start := strings.Index(body, heading)
	if start < 0 {
		return ""
	}
	rest := body[start+len(heading):]
	depth := strings.Count(strings.TrimSpace(heading), "#")

	// Fenced code blocks are skipped while looking for the end. A shell comment
	// is a "#" at the start of a line, so a section whose examples are commented
	// used to stop at the first one — and every claim after it read as absent.
	// The failure looked like missing documentation while the documentation was
	// there, which is the worst way for a drift test to be wrong.
	var (
		offset  int
		inFence bool
	)
	for _, line := range strings.SplitAfter(rest, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			offset += len(line)
			continue
		}
		if !inFence && isHeadingAtMostDepth(trimmed, depth) && offset > 0 {
			return rest[:offset]
		}
		offset += len(line)
	}
	return rest
}

// isHeadingAtMostDepth reports whether a line is a Markdown ATX heading of level
// depth or shallower, which is where a section of that depth ends.
func isHeadingAtMostDepth(line string, depth int) bool {
	hashes := 0
	for hashes < len(line) && line[hashes] == '#' {
		hashes++
	}
	if hashes == 0 || hashes > depth {
		return false
	}
	return hashes < len(line) && line[hashes] == ' '
}

// TestREADME_ShowsBothScriptFlags pins the one comparison a reader needs before
// choosing between them. They differ in what the file may contain, so an example
// of one without the other teaches half the rule.
func TestREADME_ShowsBothScriptFlags(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "README.md")
	for _, flag := range []string{"sql-file", "script-file"} {
		if !mentionsFlag(body, flag) {
			t.Errorf("README.md no longer shows --%s; the two flags are only useful explained together", flag)
		}
	}

	const heading = "### Run SQL or a sqly script from a file"
	recipe := section(body, heading)
	if recipe == "" {
		t.Fatalf("README.md no longer has the %q section", heading)
	}
	for _, flag := range []string{"sql-file", "script-file"} {
		if !mentionsFlag(recipe, flag) {
			t.Errorf("the README %q section does not show --%s", heading, flag)
		}
	}
	if !strings.Contains(recipe, "examples/") {
		t.Error("the README script section does not point at examples/, so the commands it shows are not runnable from a clone")
	}
}

// TestReference_DocumentsTheHiddenSheetFlag checks the page that promises to
// list every flag actually explains this one, rather than only naming it in a
// table row.
func TestReference_DocumentsTheHiddenSheetFlag(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "website/content/reference.md")
	if !mentionsFlag(body, "include-hidden-sheets") {
		t.Fatal("website/content/reference.md does not document --include-hidden-sheets")
	}

	const heading = "### Excel sheets"
	sheets := section(body, heading)
	if sheets == "" {
		t.Fatalf("website/content/reference.md no longer has the %q section", heading)
	}
	if !mentionsFlag(sheets, "include-hidden-sheets") {
		t.Errorf("the reference %q section does not name --include-hidden-sheets", heading)
	}
	if !strings.Contains(sheets, "only the sheets a workbook shows") {
		t.Errorf("the reference %q section does not state the default", heading)
	}
}

// TestFormats_StatesTheVisibleOnlyDefault is the check that the old contract
// cannot come back. "every sheet is imported" was true and is not, and a page
// still saying it sends a reader looking for a table that does not exist.
func TestFormats_StatesTheVisibleOnlyDefault(t *testing.T) {
	t.Parallel()

	pages := map[string]string{
		"website/content/formats.md": "",
		"doc/cookbook.md":            "",
		"README.md":                  "",
	}
	// Phrases that were true and are not. Each was in the tree when the default
	// changed, so this list is a record of what had to be corrected rather than a
	// guess at what someone might write.
	stale := []string{
		"every sheet is imported",
		"Every sheet is imported",
		"All sheets are imported",
		"all sheets are imported",
		"Every sheet becomes its own table",
		"imports every one; a hundred sheets",
		"hidden ones included",
		"imports it like any other",
	}
	for page := range pages {
		body := readDoc(t, page)
		for _, phrase := range stale {
			if strings.Contains(body, phrase) {
				t.Errorf("%s still says %q, which sqly no longer does", page, phrase)
			}
		}
	}

	formats := readDoc(t, "website/content/formats.md")
	const heading = "### Hidden sheets"
	hidden := section(formats, heading)
	if hidden == "" {
		t.Fatalf("website/content/formats.md no longer has the %q section", heading)
	}
	if !strings.Contains(hidden, "only the sheets a workbook shows") {
		t.Errorf("the formats %q section does not state the visible-only default", heading)
	}
	if !mentionsFlag(hidden, "include-hidden-sheets") {
		t.Errorf("the formats %q section does not name the flag that opts in", heading)
	}
	if !strings.Contains(hidden, "very hidden") {
		t.Errorf("the formats %q section does not say what happens to a very hidden sheet", heading)
	}
}

// TestCookbook_ShowsTheScriptFileFlag keeps the recipe that changed honest: it
// used to tell the reader to pipe a script in, which is no longer the only way.
func TestCookbook_ShowsTheScriptFileFlag(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "doc/cookbook.md")
	const heading = "## Run SQL or a sqly script from a file"
	recipe := section(body, heading)
	if recipe == "" {
		t.Fatalf("doc/cookbook.md no longer has the %q section", heading)
	}
	for _, flag := range []string{"sql-file", "script-file"} {
		if !mentionsFlag(recipe, flag) {
			t.Errorf("the cookbook %q section does not show --%s", heading, flag)
		}
	}
	if !strings.Contains(recipe, ".save") {
		t.Errorf("the cookbook %q section does not show what a script can do that SQL cannot", heading)
	}

	excel := section(body, "## Excel workbooks")
	if excel == "" {
		t.Fatal("doc/cookbook.md no longer has the \"## Excel workbooks\" section")
	}
	if !mentionsFlag(excel, "include-hidden-sheets") {
		t.Error("the cookbook Excel section does not show --include-hidden-sheets")
	}
	if !mentionsFlag(excel, "inspect") {
		t.Error("the cookbook Excel section does not point at --inspect, which is where hidden sheet names are")
	}
}

// TestDocs_SignalExitCodesAgreeWithTheImplementation ties the documented codes
// to the constants and to the arithmetic they come from, in the pages a reader
// would consult. A page saying 130 for SIGTERM would send a wrapper after the
// wrong condition.
func TestDocs_SignalExitCodesAgreeWithTheImplementation(t *testing.T) {
	t.Parallel()

	if shell.ExitInterrupt != 128+int(syscall.SIGINT) {
		t.Fatalf("ExitInterrupt = %d, want 128+SIGINT", shell.ExitInterrupt)
	}
	if shell.ExitTerminated != 128+int(syscall.SIGTERM) {
		t.Fatalf("ExitTerminated = %d, want 128+SIGTERM", shell.ExitTerminated)
	}

	pages := []string{"website/content/reference.md", "doc/cookbook.md"}
	for _, page := range pages {
		body := readDoc(t, page)
		for _, want := range []struct{ code, signal string }{
			{strconv.Itoa(shell.ExitInterrupt), "SIGINT"},
			{strconv.Itoa(shell.ExitTerminated), "SIGTERM"},
		} {
			// The code and its signal have to appear on the same line, so a page
			// that keeps the number but attaches it to the other signal fails.
			row := regexp.MustCompile("(?m)^.*`" + want.code + "`.*" + want.signal + ".*$")
			if !row.MatchString(body) {
				t.Errorf("%s does not document exit code %s as %s", page, want.code, want.signal)
			}
		}
	}
}

// TestDocs_DescribeTheDownloadLimitAsABodyLimit is the wording check that
// matters most for safety. 2 GiB bounds the bytes that arrive; it does not bound
// what importing them costs, and a page that implies otherwise invites someone
// to run an untrusted URL believing they are covered.
func TestDocs_DescribeTheDownloadLimitAsABodyLimit(t *testing.T) {
	t.Parallel()

	pages := []string{"website/content/formats.md", "doc/cookbook.md"}
	for _, page := range pages {
		body := readDoc(t, page)
		if !strings.Contains(body, "2 GiB") {
			t.Errorf("%s no longer states the 2 GiB download limit", page)
			continue
		}
		// Naming what the cap does not cover is the point; a page that only
		// states the number has said the easy half.
		for _, want := range []string{"expand", "memory"} {
			if !strings.Contains(strings.ToLower(body), want) {
				t.Errorf("%s states the 2 GiB limit without saying that %s costs are separate", page, want)
			}
		}
	}
}

// TestExamples_ExistAndAreReachable checks the runnable examples are present and
// that a reader can find them from the two documents that promise them.
func TestExamples_ExistAndAreReachable(t *testing.T) {
	t.Parallel()

	required := []string{
		"examples/README.md",
		"examples/data/sales.csv",
		"examples/data/regions.jsonl",
		"examples/join.sql",
		"examples/report.sql",
		"examples/update.sqly",
	}
	for _, path := range required {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s is missing; the documented example cannot be run from a clone: %v", path, err)
		}
	}

	if !strings.Contains(readDoc(t, "README.md"), "examples/") {
		t.Error("README.md does not link to examples/")
	}
	if !strings.Contains(readDoc(t, "doc/cookbook.md"), "/examples") {
		t.Error("doc/cookbook.md does not link to examples/")
	}
}

// TestExamples_AreRunByTheE2ESuite closes the loop the other checks leave open:
// the files exist and are linked, but only a spec that runs them proves they
// still work. Naming them in a spec is what stops examples/ becoming a museum.
func TestExamples_AreRunByTheE2ESuite(t *testing.T) {
	t.Parallel()

	const spec = "e2e/atago/examples.atago.yaml"
	body := readDoc(t, spec)
	for _, file := range []string{"report.sql", "join.sql", "update.sqly", "sales.csv", "regions.jsonl"} {
		if !strings.Contains(body, file) {
			t.Errorf("%s does not run examples/%s against the real binary", spec, file)
		}
	}
}

// TestDocs_StateTheAtomicMultiInputContract keeps the pages that promise
// all-or-nothing honest. A reader who believes a failed import leaves nothing
// behind will not go looking for half-loaded tables to clean up, so a page that
// stopped saying it — or an implementation that stopped doing it — is worth
// failing over.
func TestDocs_StateTheAtomicMultiInputContract(t *testing.T) {
	t.Parallel()

	cookbook := readDoc(t, "doc/cookbook.md")
	const cookbookHeading = "## Multiple files are one import"
	recipe := section(cookbook, cookbookHeading)
	if recipe == "" {
		t.Fatalf("doc/cookbook.md no longer has the %q section", cookbookHeading)
	}
	// Matched against the reflowed text, so a claim that spans a line wrap is
	// still found and rewrapping the paragraph is not a test failure.
	for _, want := range []string{
		"no session metadata from that import is committed",
		"temporary resources it produced are cleaned up",
		"table-name collision",
		"in the order you wrote them",
	} {
		if !strings.Contains(flatten(recipe), want) {
			t.Errorf("the cookbook %q section does not state: %s", cookbookHeading, want)
		}
	}
	// The exit code the recipe quotes has to be the one the code returns.
	if !strings.Contains(recipe, strconv.Itoa(shell.ExitInput)) {
		t.Errorf("the cookbook %q section does not name exit code %d", cookbookHeading, shell.ExitInput)
	}

	reference := readDoc(t, "website/content/reference.md")
	const referenceHeading = "### Multiple inputs"
	inputs := section(reference, referenceHeading)
	if inputs == "" {
		t.Fatalf("website/content/reference.md no longer has the %q section", referenceHeading)
	}
	for _, want := range []string{
		"every file loads or none of them",
		"refused before anything is loaded",
	} {
		if !strings.Contains(inputs, want) {
			t.Errorf("the reference %q section does not state: %s", referenceHeading, want)
		}
	}
	if !strings.Contains(inputs, "`"+strconv.Itoa(shell.ExitInput)+"`") {
		t.Errorf("the reference %q section does not name exit code %d", referenceHeading, shell.ExitInput)
	}

	// The README points at the recipe rather than repeating it.
	readme := readDoc(t, "README.md")
	if !strings.Contains(readme, "atomically") {
		t.Error("README.md no longer mentions that multiple inputs load atomically")
	}
	if !strings.Contains(readme, "multiple-files-are-one-import") {
		t.Error("README.md does not link to the cookbook recipe for it")
	}
}

// overreachingAtomicityClaims are ways of describing a failed import that
// promise more than sqly does. The guarantee is about what gets committed, not
// about what gets touched: inputs are resolved before the load, so a later URL
// may already have been downloaded and a later directory already expanded when
// an earlier input turns out to be unreadable. What holds is that nothing is
// committed and the temporary resources are released.
//
// The phrasing matters because it changes what a reader does next. "The files
// after the bad one are never read" invites them to conclude a failed import
// costs no bandwidth and leaves no temp files, and to be surprised on both
// counts. Both claims lived in the cookbook, which is why they are pinned here.
var overreachingAtomicityClaims = []string{
	"are never read",
	"is never read",
	"never reads the files after",
	"the files after the bad one",
	"later files are not read",
}

// TestREADME_CreditsTheE2ERunnerInItsOwnSection checks the README says what
// atago is and where it runs, inside the section that lists what sqly is built
// with. Two things make it worth pinning.
//
// The first is that atago is not a library sqly links: it is a test runner that
// starts the shipped binary. A heading that says "Libraries used" describes it
// wrongly, so the heading itself is part of the claim.
//
// The second is that the README mentions atago twice, here and under
// Contributing, and a whole-file search for the word cannot tell the two apart.
// Every check below reads only the section, so deleting this entry and leaving
// the Contributing mention fails.
func TestREADME_CreditsTheE2ERunnerInItsOwnSection(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "README.md")

	const oldHeading = "## Libraries used"
	if strings.Contains(body, oldHeading+"\n") {
		t.Errorf("README.md still has the %q heading; it also lists development tools, and calling atago a library misdescribes it", oldHeading)
	}

	const heading = "## Libraries and tools used"
	credits := section(body, heading)
	if credits == "" {
		t.Fatalf("README.md no longer has the %q section", heading)
	}
	if strings.Contains(credits, "## ") {
		t.Fatalf("the %q section ran past its own heading depth; the section helper is picking up the next section and every check below would pass on it", heading)
	}

	flat := flatten(credits)
	for _, want := range []struct {
		claim, why string
	}{
		{"https://github.com/nao1215/atago", "links to atago"},
		{"real `sqly` binary", "says the suite drives the shipped binary rather than a mock"},
		{"plain-YAML specs in `e2e/atago/`", "says where the specs live and what they are written in"},
		{"end-to-end", "says what kind of tests they are"},
		{"`make test-e2e`", "gives the command that runs them locally"},
		{"Linux, macOS, and Windows", "names the operating systems CI runs them on"},
		{"https://github.com/nao1215/setup-atago", "links to the action CI installs atago with"},
	} {
		if !strings.Contains(flat, want.claim) {
			t.Errorf("the README %q section does not contain %q, so it never %s", heading, want.claim, want.why)
		}
	}

	// It is a credit, not an advertisement. Two sentences was the budget.
	for _, line := range strings.Split(credits, "\n") {
		if !strings.Contains(line, "nao1215/atago") {
			continue
		}
		if got := strings.Count(line, ". ") + 1; got > 3 {
			t.Errorf("the atago entry runs to about %d sentences; keep it to a short credit and leave the details to CONTRIBUTING.md", got)
		}
	}
}

// TestREADME_E2EClaimsMatchTheWorkflow checks the README's claim about where the
// suite runs against the workflow that runs it. A README that names an OS CI
// dropped is worse than one that names none.
func TestREADME_E2EClaimsMatchTheWorkflow(t *testing.T) {
	t.Parallel()

	workflow := readDoc(t, ".github/workflows/e2e_test.yml")
	for _, runner := range []string{"ubuntu-latest", "macos-latest", "windows-latest"} {
		if !strings.Contains(workflow, runner) {
			t.Errorf("the E2E workflow no longer runs on %s, but README.md says the suite runs on Linux, macOS, and Windows", runner)
		}
	}
	if !strings.Contains(workflow, "nao1215/setup-atago") {
		t.Error("the E2E workflow no longer installs atago with setup-atago, which README.md says it does")
	}
	if !strings.Contains(workflow, "make test-e2e") {
		t.Error("the E2E workflow no longer runs `make test-e2e`, which README.md and CONTRIBUTING.md both give as the local command")
	}
}

// TestDocs_PinTheAtagoVersionCIInstalls keeps the install command contributors
// copy from lagging behind the one CI proves works. They drifted by seven
// releases before this check existed, and a contributor following the docs then
// ran the suite on a runner older than the specs it was reading.
//
// The version is written in three places — the workflow and two pages — so the
// workflow is treated as the source of truth and every `atago@vX.Y.Z` in the
// documentation is compared against it.
func TestDocs_PinTheAtagoVersionCIInstalls(t *testing.T) {
	t.Parallel()

	workflow := readDoc(t, ".github/workflows/e2e_test.yml")
	versions := regexp.MustCompile(`version:\s*(v[0-9]+\.[0-9]+\.[0-9]+)`).FindAllStringSubmatch(workflow, -1)
	if len(versions) == 0 {
		t.Fatal("no atago version is pinned in .github/workflows/e2e_test.yml; the check below has nothing to compare against")
	}
	want := versions[0][1]
	for _, v := range versions {
		if v[1] != want {
			t.Errorf("the E2E workflow installs both %s and %s; the documentation can only name one", want, v[1])
		}
	}

	installed := regexp.MustCompile(`atago@(v[0-9]+\.[0-9]+\.[0-9]+)`)
	for _, page := range []string{"CONTRIBUTING.md", "doc/build_and_test.md", "README.md"} {
		found := installed.FindAllStringSubmatch(readDoc(t, page), -1)
		for _, m := range found {
			if m[1] != want {
				t.Errorf("%s tells contributors to install atago@%s, but CI runs the suite with %s", page, m[1], want)
			}
		}
		if page != "README.md" && len(found) == 0 {
			t.Errorf("%s no longer shows how to install atago; it is not part of `make tools`, so a contributor has no other way to learn the version", page)
		}
	}
}

// TestBuild_DoesNotDependOnGoogleWire keeps an archived dependency from coming
// back one file at a time.
//
// sqly's application wiring is a hand-written composition root (`di/di.go`).
// Google Wire generated it until v1.0.0-rc2, and the pieces that made it work
// were spread across the build: a module requirement, a tool import, an install
// line, and an architecture vendor. Restoring any one of them alone builds
// fine — `tools.go` is behind a build tag, an unused vendor is not an error —
// so nothing else in the tree would notice a partial return.
//
// `github.com/moov-io/wire` is a different project entirely: it is Fedwire
// support, it arrives through filesql, and sqly reads `.fed` files with it. The
// checks below name Google's module in full so a search-and-destroy for "wire"
// cannot take it out.
func TestBuild_DoesNotDependOnGoogleWire(t *testing.T) {
	t.Parallel()

	const googleWire = "github.com/google/wire"
	for _, file := range []string{"go.mod", "go.sum", "tools.go", "Makefile", ".go-arch-lint.yml"} {
		body := readDoc(t, file)
		for _, line := range strings.Split(body, "\n") {
			if strings.Contains(line, googleWire) {
				t.Errorf("%s still references %s:\n\t%s\nApplication wiring is hand-written in di/di.go; Wire's repository is archived.", file, googleWire, strings.TrimSpace(line))
			}
		}
	}

	// The Fedwire library shares a name and must survive. It reaches sqly through
	// filesql, so it is an indirect requirement rather than a direct one.
	if !strings.Contains(readDoc(t, "go.sum"), "github.com/moov-io/wire") {
		t.Error("github.com/moov-io/wire is gone from go.sum; that is the Fedwire library behind .fed support, not Google Wire")
	}
}

// TestDocs_DoNotDescribeAtomicityAsAvoidedIO fails when any documentation page
// describes the import contract as I/O that does not happen.
func TestDocs_DoNotDescribeAtomicityAsAvoidedIO(t *testing.T) {
	t.Parallel()

	pages := []string{"README.md", "doc/cookbook.md", "CHANGELOG.md"}
	entries, err := os.ReadDir("website/content")
	if err != nil {
		t.Fatalf("read website/content: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			pages = append(pages, filepath.Join("website", "content", entry.Name()))
		}
	}

	for _, page := range pages {
		body := strings.ToLower(readDoc(t, page))
		for _, claim := range overreachingAtomicityClaims {
			if strings.Contains(body, claim) {
				t.Errorf("%s says %q; atomicity is a promise about what is committed and cleaned up, not about which inputs are touched — inputs are resolved before the load, so a later one may already have been downloaded", page, claim)
			}
		}
	}
}

// TestE2E_ExercisesTheMultiInputContract closes the loop the wording checks
// leave open: the pages say all-or-nothing, and a spec has to run it against the
// real binary — including with the unreadable file in the middle and at the end,
// which are the only positions that can show a rollback happened.
func TestE2E_ExercisesTheMultiInputContract(t *testing.T) {
	t.Parallel()

	const spec = "e2e/atago/multi_input.atago.yaml"
	body := readDoc(t, spec)
	for _, want := range []string{
		"a broken input first",
		"a broken input in the middle",
		"a broken input last",
		"table-name collision",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("%s does not cover: %s", spec, want)
		}
	}
}

// The rc3 agent-safety contracts, guarded against drift.
//
// Each of the three — schema-only --inspect, default-deny remote input, and the
// dialect warning — is a promise made in code and repeated in several documents.
// A promise repeated in four places drifts in three of them, and the failure
// mode is the worst kind: a page that reads correctly and describes a tool that
// no longer behaves that way. The tests below derive the claim from the
// implementation wherever they can, so the documentation is checked against the
// code rather than against a copy of itself.

// inspectSchemaFile is the single canonical copy of the --inspect JSON contract.
// It is published by Hugo as /sqly/schema/inspect-v1.schema.json; nothing else in
// the repository restates it.
const inspectSchemaFile = "website/static/schema/inspect-v1.schema.json"

// TestInspect_SchemaVersionAgreesEverywhere ties the three places the contract
// version is written down to one number.
func TestInspect_SchemaVersionAgreesEverywhere(t *testing.T) {
	t.Parallel()

	if shell.InspectSchemaVersion != 1 {
		t.Fatalf("shell.InspectSchemaVersion = %d, want 1; v1 is the version this release publishes",
			shell.InspectSchemaVersion)
	}

	schema := readDoc(t, inspectSchemaFile)
	var parsed struct {
		Schema     string `json:"$schema"` //nolint:tagliatelle // a JSON Schema keyword, not a name sqly chooses
		Properties struct {
			SchemaVersion struct {
				Const *int `json:"const"`
			} `json:"schema_version"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("%s is not valid JSON: %v", inspectSchemaFile, err)
	}
	if parsed.Properties.SchemaVersion.Const == nil {
		t.Fatalf("%s does not pin schema_version to a constant", inspectSchemaFile)
	}
	if *parsed.Properties.SchemaVersion.Const != shell.InspectSchemaVersion {
		t.Errorf("%s pins schema_version to %d, the implementation emits %d",
			inspectSchemaFile, *parsed.Properties.SchemaVersion.Const, shell.InspectSchemaVersion)
	}
	// The draft has to be stated, or "valid against the schema" means whatever
	// the validator happened to assume.
	if !strings.Contains(parsed.Schema, "2020-12") {
		t.Errorf("%s declares $schema %q; the documented draft is 2020-12", inspectSchemaFile, parsed.Schema)
	}
	for _, field := range []string{"schema_version", "sqly_version", "tables"} {
		if !slices.Contains(parsed.Required, field) {
			t.Errorf("%s does not require the top-level field %q", inspectSchemaFile, field)
		}
	}

	// The reference page states the same number to a reader, in the section that
	// explains it rather than anywhere in the file.
	body := readDoc(t, "website/content/reference.md")
	policy := section(body, "### Inspect JSON schema")
	if policy == "" {
		t.Fatal("website/content/reference.md has no \"Inspect JSON schema\" section")
	}
	want := "It is `" + strconv.Itoa(shell.InspectSchemaVersion) + "`."
	if !strings.Contains(flatten(policy), want) {
		t.Errorf("the reference's schema section does not state %q; it must name the version the implementation emits", want)
	}
}

// TestInspectSchema_KeepsSampleRowsRequiredAndTyped guards the two properties
// the schema exists to hold: sample_rows is a required array, and the objects
// stay open so an additive field does not force a version bump.
func TestInspectSchema_KeepsSampleRowsRequiredAndTyped(t *testing.T) {
	t.Parallel()

	// additionalProperties is decoded as `any` because JSON Schema allows both a
	// boolean and a subschema there, and the sample-row object legitimately uses
	// the subschema form to type its values. Only a literal false is a closed
	// object.
	var parsed struct {
		Defs map[string]struct {
			Required             []string `json:"required"`
			AdditionalProperties any      `json:"additionalProperties"` //nolint:tagliatelle // a JSON Schema keyword
			Properties           map[string]struct {
				Type any `json:"type"`
			} `json:"properties"`
		} `json:"$defs"` //nolint:tagliatelle // a JSON Schema keyword
	}
	if err := json.Unmarshal([]byte(readDoc(t, inspectSchemaFile)), &parsed); err != nil {
		t.Fatalf("%s is not valid JSON: %v", inspectSchemaFile, err)
	}

	table, ok := parsed.Defs["table"]
	if !ok {
		t.Fatal("the schema has no table definition")
	}
	if !slices.Contains(table.Required, "sample_rows") {
		t.Error("sample_rows is not required; the contract is that it is always present, empty rather than absent")
	}
	if kind, ok := table.Properties["sample_rows"].Type.(string); !ok || kind != "array" {
		t.Errorf("sample_rows is typed %v, want \"array\" (never null)", table.Properties["sample_rows"].Type)
	}
	for name, def := range parsed.Defs {
		if closed, ok := def.AdditionalProperties.(bool); ok && !closed {
			t.Errorf("%s closes itself with additionalProperties:false; v1 promises additive fields do not move the version", name)
		}
	}
}

// TestInspect_DefaultSampleIsZeroInCodeHelpAndReference is the drift guard for
// the "default 5" that used to be true. It derives the number from the parser,
// so the documentation is compared to the implementation rather than to itself.
func TestInspect_DefaultSampleIsZeroInCodeHelpAndReference(t *testing.T) {
	t.Parallel()

	if config.DefaultInspectSample != 0 {
		t.Fatalf("config.DefaultInspectSample = %d, want 0", config.DefaultInspectSample)
	}

	arg, err := config.NewArg([]string{"sqly"})
	if err != nil {
		t.Fatalf("NewArg: %v", err)
	}
	help := flatten(arg.Usage)
	if !strings.Contains(help, "--inspect-sample N") {
		t.Fatal("--help does not list --inspect-sample")
	}
	if !strings.Contains(help, "(default: 0)") {
		t.Errorf("--help does not show a flag defaulting to 0; --inspect-sample must advertise its own default:\n%s", arg.Usage)
	}

	inspection := section(readDoc(t, "website/content/reference.md"), "## Inspection")
	if inspection == "" {
		t.Fatal("website/content/reference.md has no Inspection section")
	}
	flat := flatten(inspection)
	if !strings.Contains(flat, "default `0`") {
		t.Errorf("the reference's Inspection section does not state the default of 0")
	}
	// The old default must be gone from the section, not merely outnumbered.
	for _, stale := range []string{"default 5", "(default 5", "default `5`"} {
		if strings.Contains(flat, stale) {
			t.Errorf("the reference's Inspection section still says %q; the default is 0", stale)
		}
	}
	if !strings.Contains(flat, "schema-only by default") {
		t.Error("the reference's Inspection section does not say --inspect is schema-only by default")
	}
	for _, claim := range []string{"`schema_version`", "`sqly_version`"} {
		if !strings.Contains(flat, claim) {
			t.Errorf("the reference's Inspection section does not document %s", claim)
		}
	}
}

// TestReference_LinksTheFormalInspectSchema keeps the canonical file reachable
// from the page that explains it. A schema nobody can find is a schema nobody
// validates against.
func TestReference_LinksTheFormalInspectSchema(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(inspectSchemaFile); err != nil {
		t.Fatalf("the canonical schema is missing: %v", err)
	}
	body := readDoc(t, "website/content/reference.md")
	if !strings.Contains(body, "schema/inspect-v1.schema.json") {
		t.Error("the reference does not link the published JSON Schema")
	}

	// Hugo replaces its implicit static mount as soon as any module mount is
	// declared, so the site only publishes the schema if the mount is named.
	hugo := readDoc(t, "website/hugo.toml")
	if !strings.Contains(hugo, `source = "static"`) {
		t.Error("website/hugo.toml does not mount the static directory, so the schema would not be published")
	}
}

// TestInspectSchema_CompatibilityPolicyIsStated checks the policy a consumer
// needs before it can rely on the version number at all: which changes move it
// and which do not.
func TestInspectSchema_CompatibilityPolicyIsStated(t *testing.T) {
	t.Parallel()

	policy := flatten(section(readDoc(t, "website/content/reference.md"), "#### Compatibility policy for schema version 1"))
	if policy == "" {
		t.Fatal("the reference has no compatibility policy section for the inspect schema")
	}
	for _, claim := range []string{
		"must ignore fields it does not know",
		"adding an optional top-level field",
		"removing a field",
		"renaming a field",
		"changing a field's type",
		"changing whether a field can be `null`",
		"making `sample_rows` optional again",
	} {
		if !strings.Contains(policy, claim) {
			t.Errorf("the compatibility policy does not state: %s", claim)
		}
	}
}

// TestDocs_EveryRemoteExampleGrantsTheCapability walks every documented sqly
// command and requires --allow-remote on the ones that name an http(s) input.
// It is derived from the commands themselves rather than from a list of files,
// so an example added to any page is covered.
func TestDocs_EveryRemoteExampleGrantsTheCapability(t *testing.T) {
	t.Parallel()

	checked := 0
	counterExamples := 0
	for _, doc := range docSources(t) {
		exempt := refusalExampleLines(t, doc)
		for _, cmd := range shellCommandsIn(t, doc) {
			args, ok := sqlyInvocation(cmd.text)
			if !ok {
				continue
			}
			remote := false
			allowed := false
			for _, arg := range args {
				if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
					remote = true
				}
				if arg == "--allow-remote" {
					allowed = true
				}
			}
			if !remote {
				continue
			}
			if exempt[cmd.line] {
				// A deliberate counter-example: the documentation is showing the
				// command that gets refused, next to the one that works.
				counterExamples++
				continue
			}
			checked++
			if !allowed {
				t.Errorf("%s:%d runs sqly against a URL without --allow-remote, which the current CLI refuses:\n  %s",
					doc, cmd.line, cmd.text)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no documented sqly command names a URL; either the examples were removed or this parser stopped matching them")
	}
	if counterExamples == 0 {
		t.Error("no documentation shows the refused form; a reader who has never seen the refusal cannot recognize it")
	}
}

// refusalExampleLines returns the lines of a document holding a command that the
// prose deliberately shows being refused. Such a command is marked by a shell
// comment naming the refusal on the line before it, so a counter-example is
// opt-in and visible in the source rather than inferred.
func refusalExampleLines(t *testing.T, path string) map[int]bool {
	t.Helper()

	exempt := make(map[int]bool)
	inShell := false
	marked := false
	for i, raw := range strings.Split(readDoc(t, path), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inShell = strings.TrimPrefix(line, "```") == "shell"
			marked = false
			continue
		}
		if !inShell || line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			marked = strings.Contains(strings.ToLower(line), "refused")
			continue
		}
		if marked {
			exempt[i+1] = true
			marked = false
		}
	}
	return exempt
}

// TestDemoTapes_EveryRemoteInvocationGrantsTheCapability covers the recorded
// demos, whose commands the parser check above does not see.
func TestDemoTapes_EveryRemoteInvocationGrantsTheCapability(t *testing.T) {
	t.Parallel()

	tapes, err := filepath.Glob("doc/vhs/*.tape")
	if err != nil {
		t.Fatalf("glob the tapes: %v", err)
	}
	for _, tape := range tapes {
		for _, cmd := range tapeCommands(t, tape) {
			args, ok := sqlyInvocation(cmd.text)
			if !ok {
				continue
			}
			remote := false
			for _, arg := range args {
				if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
					remote = true
				}
			}
			if !remote {
				continue
			}
			if !slices.Contains(args, "--allow-remote") {
				t.Errorf("%s:%d records a URL invocation without --allow-remote, so the GIF would show a refusal:\n  %s",
					tape, cmd.line, cmd.text)
			}
		}
	}
}

// TestDocs_StateTheRemoteDefaultDeny checks the claim itself, not only the flag
// name. A page listing --allow-remote among the flags while still describing a
// URL as something sqly just fetches teaches the old contract.
func TestDocs_StateTheRemoteDefaultDeny(t *testing.T) {
	t.Parallel()

	formats := flatten(section(readDoc(t, "website/content/formats.md"), "## Remote inputs"))
	if formats == "" {
		t.Fatal("the formats page has no Remote inputs section")
	}
	for _, claim := range []string{
		"default-deny",
		"`--allow-remote`",
		"before any request",
	} {
		if !strings.Contains(formats, claim) {
			t.Errorf("the formats page's Remote inputs section does not state: %s", claim)
		}
	}

	// The capability must be described by what it is not, in the same place. A
	// reader who takes it for an SSRF defense is worse off than one who has never
	// heard of it.
	notASandbox := flatten(section(readDoc(t, "website/content/formats.md"), "### What --allow-remote is not"))
	if notASandbox == "" {
		t.Fatal("the formats page does not say what --allow-remote is not")
	}
	for _, claim := range []string{
		"not a sandbox",
		"SSRF",
		"localhost",
		"private network",
		"metadata endpoint",
		"DNS rebinding",
	} {
		if !strings.Contains(notASandbox, claim) {
			t.Errorf("the capability's limits section does not mention: %s", claim)
		}
	}

	// The limits it does not lift stay documented alongside it.
	remote := flatten(readDoc(t, "website/content/formats.md"))
	for _, claim := range []string{"2 GiB", "Redirect scheme", "cannot be written back"} {
		if !strings.Contains(remote, claim) {
			t.Errorf("the formats page no longer states: %s", claim)
		}
	}
}

// TestREADME_KeepsTheRemoteNoteShortAndCorrect checks the README says the two
// things a reader needs and links out for the rest, rather than restating the
// formats page.
func TestREADME_KeepsTheRemoteNoteShortAndCorrect(t *testing.T) {
	t.Parallel()

	body := flatten(readDoc(t, "README.md"))
	for _, claim := range []string{
		"A URL needs `--allow-remote`",
		"not a sandbox or an SSRF defense",
		"formats/#remote-inputs",
	} {
		if !strings.Contains(body, claim) {
			t.Errorf("README does not state: %s", claim)
		}
	}
}

// TestHelp_DocumentsTheRc3Contracts checks --help itself, which is the one
// document a user reads without a browser.
func TestHelp_DocumentsTheRc3Contracts(t *testing.T) {
	t.Parallel()

	arg, err := config.NewArg([]string{"sqly"})
	if err != nil {
		t.Fatalf("NewArg: %v", err)
	}
	help := flatten(arg.Usage)

	for _, claim := range []string{
		"--allow-remote",
		"not a sandbox or an SSRF defense",
		"downloaded only with --allow-remote",
		"no row data unless --inspect-sample asks for it",
		"does not emulate that database",
		"SQLite semantics",
		"says so once, on stderr",
	} {
		if !strings.Contains(help, claim) {
			t.Errorf("--help does not state: %s\n\n%s", claim, arg.Usage)
		}
	}
}

// TestDialectDocs_StateSQLiteSemanticsAndTheWarning keeps the dialect page's
// existing "translation, not emulation" claim and adds the runtime behavior to
// it, so a reader learns both from the same section.
func TestDialectDocs_StateSQLiteSemanticsAndTheWarning(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "website/content/dialects.md")
	if !strings.Contains(body, "## This is translation, not emulation") {
		t.Fatal("the dialects page lost its \"translation, not emulation\" section")
	}
	translation := flatten(section(body, "## This is translation, not emulation"))
	for _, claim := range []string{
		"once, on stderr",
		"SQLite semantics",
		"does not emulate the source database's semantics",
		"never to stdout",
	} {
		if !strings.Contains(translation, claim) {
			t.Errorf("the dialects page does not state: %s", claim)
		}
	}
}

// TestGoReleaser_DescriptionsNameTheCurrentFormats catches the release metadata
// drifting behind what sqly reads. It was advertising four formats and
// "Microsoft Excel™" while sqly read nine.
func TestGoReleaser_DescriptionsNameTheCurrentFormats(t *testing.T) {
	t.Parallel()

	config := readDoc(t, ".goreleaser.yml")

	// The release header is the first thing a visitor to a release page reads.
	header := flatten(config)
	for _, format := range []string{"CSV", "TSV", "LTSV", "JSON", "JSONL", "Parquet", "Excel", "ACH", "Fedwire"} {
		if !strings.Contains(header, format) {
			t.Errorf(".goreleaser.yml's release header does not name %s", format)
		}
	}
	if !strings.Contains(strings.ToLower(header), "compress") {
		t.Error(".goreleaser.yml's release header does not mention compressed inputs")
	}

	// Both package descriptions — nFPM and Homebrew — must have moved off the old
	// four-format list. They are checked by absence of the stale phrasing and by
	// presence of the formats that were missing from it.
	descriptions := regexp.MustCompile(`(?m)^\s*description:\s*(.+)$`).FindAllStringSubmatch(config, -1)
	if len(descriptions) != 2 {
		t.Fatalf("found %d description fields in .goreleaser.yml, want 2 (nFPM and Homebrew)", len(descriptions))
	}
	for i, match := range descriptions {
		desc := strings.TrimSpace(match[1])
		if strings.Contains(desc, "Microsoft Excel") {
			t.Errorf("description %d still uses the old format list: %q", i+1, desc)
		}
		for _, format := range []string{"JSONL", "Parquet", "ACH", "Fedwire", "compressed"} {
			if !strings.Contains(desc, format) {
				t.Errorf("description %d does not name %s: %q", i+1, format, desc)
			}
		}
	}

	// Homebrew rejects a formula whose desc runs past 80 characters, so the
	// shorter of the two has to stay short. The second description is the brews
	// one; the file's order is asserted above by there being exactly two.
	brew := strings.TrimSpace(descriptions[1][1])
	if len(brew) > 80 {
		t.Errorf("the Homebrew description is %d characters; brew audit caps it at 80: %q", len(brew), brew)
	}
}

// TestDocs_NoLinkToADeletedDesignDocument keeps a link to internal design prose
// from creeping back into published documentation.
//
// The two documents this checks for were deleted because prose about internal
// layering has to be maintained by hand beside the code it describes, and one of
// them had drifted far enough to describe a program sqly no longer is, with
// three missing images, while the public about page still sent readers to it.
// The layering is checked by go-arch-lint instead, which cannot drift.
func TestDocs_NoLinkToADeletedDesignDocument(t *testing.T) {
	t.Parallel()

	deleted := map[string]bool{
		"doc/architecture.md":    true,
		"doc/design_overview.md": true,
	}

	for _, path := range append(docSources(t), "CONTRIBUTING.md", "doc/build_and_test.md") {
		for _, target := range markdownLinkTargets(readDoc(t, path)) {
			if deleted[resolveDocLink(path, target)] {
				t.Errorf("%s links %s, which was deleted; the layering is documented by .go-arch-lint.yml", path, target)
			}
		}
	}
}

// markdownLinkPattern matches the target of a Markdown inline link.
var markdownLinkPattern = regexp.MustCompile(`\]\(([^)\s]+)`)

// markdownLinkTargets returns every inline link target in a document, with any
// fragment stripped.
func markdownLinkTargets(doc string) []string {
	matches := markdownLinkPattern.FindAllStringSubmatch(doc, -1)
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		target, _, _ := strings.Cut(m[1], "#")
		if target != "" {
			targets = append(targets, target)
		}
	}
	return targets
}

// resolveDocLink turns a link target into the repository path it points at, so a
// document under doc/ writing "architecture.md" and the README writing
// "./doc/architecture.md" resolve to the same file. A link to this repository on
// GitHub is resolved by the path after the branch, since that is the same file
// seen through a URL. Anything else (an external site, a mailto) resolves to
// itself and matches nothing.
func resolveDocLink(from, target string) string {
	const blob = "github.com/nao1215/sqly/blob/main/"
	if i := strings.Index(target, blob); i >= 0 {
		return target[i+len(blob):]
	}
	if strings.Contains(target, "://") {
		return target
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(from), target)))
}

// TestDocs_NoLinkToTheDeletedMigrationGuide keeps a pointer to the migration
// guide from coming back.
//
// The guide restated the CHANGELOG's Breaking Changes entries as before-and-after
// commands, so every breaking change was written twice and kept in step by hand.
// A breaking change is now described once, in its entry. A link left behind, or
// added back, points at a file that is not there.
//
// The CHANGELOG is checked too, and it is the reason this resolves links rather
// than matching text: a frozen entry may still name the guide as something a past
// release added, so a mention is allowed where a link is not.
func TestDocs_NoLinkToTheDeletedMigrationGuide(t *testing.T) {
	t.Parallel()

	for _, path := range append(docSources(t), "CONTRIBUTING.md", "doc/build_and_test.md", "CHANGELOG.md") {
		for _, target := range markdownLinkTargets(readDoc(t, path)) {
			if resolveDocLink(path, target) == "doc/migration.md" {
				t.Errorf("%s links %s, which was deleted; a breaking change is described in its CHANGELOG entry", path, target)
			}
		}
	}
}

// TestAbout_BenchmarkIsMarkedHistorical keeps an old measurement from reading as
// a promise about the current release.
func TestAbout_BenchmarkIsMarkedHistorical(t *testing.T) {
	t.Parallel()

	about := flatten(section(readDoc(t, "website/content/about.md"), "## Benchmark"))
	if about == "" {
		t.Fatal("the about page has no Benchmark section")
	}
	for _, claim := range []string{
		"Historical measurement",
		"v0.30.0",
		"not a performance guarantee",
	} {
		if !strings.Contains(about, claim) {
			t.Errorf("the about page's Benchmark section does not state: %s", claim)
		}
	}

	readme := flatten(section(readDoc(t, "README.md"), "## Benchmark"))
	if readme == "" {
		t.Fatal("the README has no Benchmark section")
	}
	for _, claim := range []string{"historical measurement", "not a performance guarantee"} {
		if !strings.Contains(readme, claim) {
			t.Errorf("the README's Benchmark section does not state: %s", claim)
		}
	}
}

// TestCHANGELOG_ListsTheRc7BreakingChanges holds the release notes to the three
// surfaces rc7 took away.
//
// Each is something a user could type into rc6 and cannot type into rc7, so the
// notes are the only place they learn it before the shell tells them. A removal
// that reaches a release without an entry here is the failure this catches.
func TestCHANGELOG_ListsTheRc7BreakingChanges(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "CHANGELOG.md")
	rc7 := section(body, "## [v1.0.0-rc7]")
	if rc7 == "" {
		t.Fatal("CHANGELOG.md has no v1.0.0-rc7 section")
	}
	breaking := flatten(section(rc7, "### Breaking Changes"))
	if breaking == "" {
		t.Fatal("the v1.0.0-rc7 CHANGELOG entry has no Breaking Changes section")
	}
	for _, claim := range []string{
		"`.header` is removed",
		"`.mode` no longer takes `excel` or `parquet`",
		"SQLY_HISTORY_PATH",
		// The replacement matters as much as the removal: an entry that says a
		// command is gone without saying what to type instead leaves the reader
		// where the error message already left them.
		"`.describe`",
		".dump TABLE out.xlsx",
	} {
		if !strings.Contains(breaking, claim) {
			t.Errorf("the rc7 Breaking Changes section does not mention %s", claim)
		}
	}
}

// TestCHANGELOG_ListsTheRc6BreakingChanges is the rc5 check for the release
// after it: what the guide tells someone to change, the release notes have to
// record.
func TestCHANGELOG_ListsTheRc6BreakingChanges(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "CHANGELOG.md")
	rc6 := section(body, "## [v1.0.0-rc6]")
	if rc6 == "" {
		t.Fatal("CHANGELOG.md has no v1.0.0-rc6 section")
	}
	breaking := flatten(section(rc6, "### Breaking Changes"))
	if breaking == "" {
		t.Fatal("the v1.0.0-rc6 CHANGELOG entry has no Breaking Changes section")
	}
	for _, claim := range []string{"out.csv.bz2", "last row is empty", "Infinity", "`4`"} {
		if !strings.Contains(breaking, claim) {
			t.Errorf("the rc6 Breaking Changes section does not mention %s", claim)
		}
	}
}

// TestCHANGELOG_ListsTheRc5BreakingChanges is the rc4 check for the release
// after it: what the guide tells someone to change, the release notes have to
// record.
func TestCHANGELOG_ListsTheRc5BreakingChanges(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "CHANGELOG.md")
	rc5 := section(body, "## [v1.0.0-rc5]")
	if rc5 == "" {
		t.Fatal("CHANGELOG.md has no v1.0.0-rc5 section")
	}
	breaking := flatten(section(rc5, "### Breaking Changes"))
	if breaking == "" {
		t.Fatal("the v1.0.0-rc5 CHANGELOG entry has no Breaking Changes section")
	}
	for _, claim := range []string{"Excel", "U+FFFE", "source", "user:xxxxx@"} {
		if !strings.Contains(breaking, claim) {
			t.Errorf("the rc5 Breaking Changes section does not mention %s", claim)
		}
	}
}

// TestCHANGELOG_ListsTheRc4BreakingChanges is the rc3 check for the release
// after it: what the guide tells someone to change, the release notes have to
// record.
func TestCHANGELOG_ListsTheRc4BreakingChanges(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "CHANGELOG.md")
	rc4 := section(body, "## [v1.0.0-rc4]")
	if rc4 == "" {
		t.Fatal("CHANGELOG.md has no v1.0.0-rc4 section")
	}
	breaking := flatten(section(rc4, "### Breaking Changes"))
	if breaking == "" {
		t.Fatal("the v1.0.0-rc4 CHANGELOG entry has no Breaking Changes section")
	}
	for _, claim := range []string{".dialect", ".row-mismatch", "--inspect", "--dialect"} {
		if !strings.Contains(breaking, claim) {
			t.Errorf("the rc4 Breaking Changes section does not mention %s", claim)
		}
	}
}

// TestCHANGELOG_ListsTheRc3BreakingChanges keeps the release notes in step with
// the guide, since the two are read by different people.
func TestCHANGELOG_ListsTheRc3BreakingChanges(t *testing.T) {
	t.Parallel()

	body := readDoc(t, "CHANGELOG.md")
	rc3 := section(body, "## [v1.0.0-rc3]")
	if rc3 == "" {
		t.Fatal("CHANGELOG.md has no v1.0.0-rc3 section")
	}
	breaking := flatten(section(rc3, "### Breaking Changes"))
	if breaking == "" {
		t.Fatal("the v1.0.0-rc3 CHANGELOG entry has no Breaking Changes section")
	}
	for _, claim := range []string{"--allow-remote", "--inspect-sample", "schema_version"} {
		if !strings.Contains(breaking, claim) {
			t.Errorf("the rc3 Breaking Changes section does not mention %s", claim)
		}
	}
}

// pagesVerifyRequire matches the `require "${page}" "claim" "where"` lines the
// website workflow checks the live site with.
var pagesVerifyRequire = regexp.MustCompile(`(?m)^\s*require "\$\{(\w+)\}" "([^"]*)" "(\w+)"`)

// pagesVerifySources maps the page name the workflow uses to the Markdown it is
// rendered from, so a claim can be checked against the source before a deploy
// has to check it against the live site.
var pagesVerifySources = map[string]string{
	"reference": "website/content/reference.md",
	"formats":   "website/content/formats.md",
	"dialects":  "website/content/dialects.md",
	"about":     "website/content/about.md",
}

// TestPagesVerification_EveryClaimIsInTheSourceItChecks runs the deploy check's
// own claim list against the Markdown those pages are built from.
//
// It exists because the deploy check is the one test that cannot run before a
// deploy: a claim reworded out of a page, or a needle typed slightly wrong,
// only fails after the site is already live. Comparing the list to the source
// moves that failure to `go test`.
func TestPagesVerification_EveryClaimIsInTheSourceItChecks(t *testing.T) {
	t.Parallel()

	workflow := readDoc(t, ".github/workflows/website.yml")
	matches := pagesVerifyRequire.FindAllStringSubmatch(workflow, -1)
	if len(matches) == 0 {
		t.Fatal("no require lines parsed from the website workflow; the parser or the workflow changed")
	}

	flattened := make(map[string]string, len(pagesVerifySources))
	for page, path := range pagesVerifySources {
		flattened[page] = flatten(readDoc(t, path))
	}

	for _, m := range matches {
		page, claim := m[1], m[2]
		source, known := flattened[page]
		if !known {
			t.Errorf("the workflow checks a page %q this test cannot map to a Markdown source; add it to pagesVerifySources", page)
			continue
		}
		if !strings.Contains(source, claim) {
			t.Errorf("%s no longer states %q, which the Pages verification requires of the deployed page",
				pagesVerifySources[page], claim)
		}
	}
}

// TestPagesVerification_NormalizesWhitespaceBeforeMatching keeps the workflow
// from going back to a raw substring match on the fetched HTML.
//
// Every step that searches a page for prose must squeeze whitespace first.
// Without it, a claim that happens to wrap in the Markdown source is absent
// from the string being searched, and the deploy fails on a page that says
// exactly the right thing — which is what happened the first time this check
// ran.
//
// The steps are named rather than every curl being matched, because the
// workflow fetches for two other reasons that are unaffected: the homepage is
// read to pull a 40-hex build commit out of a meta tag, and the schema is
// fetched to a file and parsed as JSON. Neither searches prose.
func TestPagesVerification_NormalizesWhitespaceBeforeMatching(t *testing.T) {
	t.Parallel()

	const squeeze = `tr -s '[:space:]' ' '`
	workflow := readDoc(t, ".github/workflows/website.yml")

	for _, step := range []string{
		"Check the reference page shows the current flags",
		"Check the deployed pages state the contracts a wrong page would break",
	} {
		body := workflowStep(t, workflow, step)
		if body == "" {
			t.Errorf("the website workflow has no step named %q", step)
			continue
		}
		if !strings.Contains(body, squeeze) {
			t.Errorf("the %q step does not normalize whitespace with %s, so a claim that wraps in the source would not match",
				step, squeeze)
		}
	}
}

// workflowStep returns the body of a named workflow step, or "" when there is
// none. A step ends where the next one begins.
func workflowStep(t *testing.T, workflow, name string) string {
	t.Helper()

	start := strings.Index(workflow, "- name: "+name)
	if start < 0 {
		return ""
	}
	rest := workflow[start+len("- name: "+name):]
	if end := strings.Index(rest, "\n      - name: "); end >= 0 {
		return rest[:end]
	}
	return rest
}

// TestPagesVerification_ChecksTheRc3Contracts guards the deploy check itself.
// The website workflow verifies the live site against a list of claims; a
// contract added without being added there is a contract the deploy cannot
// catch drifting.
func TestPagesVerification_ChecksTheRc3Contracts(t *testing.T) {
	t.Parallel()

	workflow := readDoc(t, ".github/workflows/website.yml")
	for _, claim := range []string{
		"--allow-remote",
		"schema-only by default",
		"default-deny",
		"not a sandbox or an SSRF defense",
		"schema/inspect-v1.schema.json",
		"Historical measurement",
	} {
		if !strings.Contains(workflow, claim) {
			t.Errorf("the Pages verification does not check the deployed site for: %s", claim)
		}
	}
}

// TestDocs_QuotedMessagesAreTheOnesTheBinaryPrints ties every message quoted
// verbatim in the documentation to the E2E scenario that asserts sqly prints it.
//
// It exists because a substring assertion is not enough to hold a quote in
// place. e2e/atago/script_contract.atago.yaml checked only `contains: "runs SQL
// only"`, so the reference kept advertising a sentence sqly had stopped
// printing — a different flag to reach for and a `printf '...' | sqly FILE`
// recipe that had been replaced by --script-file — and every test stayed green.
//
// Both sides hold the same string, so neither can move alone: editing the
// documentation fails here, and changing the message fails the E2E that runs the
// real binary. A message worth quoting is worth pinning in both places; a
// message not pinned in an E2E should not be quoted as if it were what sqly
// says.
func TestDocs_QuotedMessagesAreTheOnesTheBinaryPrints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// message is quoted verbatim by both files below.
		message string
		doc     string
		spec    string
	}{
		{
			name:    "a helper command in a --sql-file",
			message: `save.sql runs SQL only, but line 2 is the helper command ".save"; run it with --script-file, or pipe it to sqly`,
			doc:     referencePage,
			spec:    "e2e/atago/script_contract.atago.yaml",
		},
		{
			name:    "the notice for sheets an import left behind",
			message: "Skipped 2 hidden sheets in book.xlsx; start sqly with --include-hidden-sheets to import them.",
			doc:     referencePage,
			spec:    "e2e/atago/excel_sheets.atago.yaml",
		},
		{
			name:    "the hint that lists the tables a session has",
			message: `hint: this session has no table "staf". Available tables: ident, staff. sqly derives table names from file names: https://nao1215.github.io/sqly/reference/#table-name-rules`,
			doc:     referencePage,
			spec:    "e2e/atago/error_hygiene.atago.yaml",
		},
		{
			// The path in the full message is the workdir's, so what is pinned is
			// the advice, which is the part a reader acts on.
			name:    "the refusal to save in place through a symlink",
			message: "an in-place save would overwrite that file, which you did not name. Add --follow-symlinks to do it anyway, or save to a directory with .save DIR",
			doc:     referencePage,
			spec:    "e2e/atago/file_write_contract.atago.yaml",
		},
		{
			name:    "the import stopped by the default row-mismatch policy",
			message: "failed to import file rm.csv: filesql: column count mismatch: row 1 has 2 fields, want 3; use --row-mismatch skip to drop such rows, or --row-mismatch pad to fill short ones",
			doc:     "website/content/formats.md",
			spec:    "e2e/atago/error_hygiene.atago.yaml",
		},
		{
			name:    "the construct a dialect rejects rather than passing to SQLite",
			message: "translate error (postgresql): dialect: syntax not supported on SQLite backend: DISTINCT ON is not supported: SELECT DISTINCT ON (g) g, v FROM t",
			doc:     "website/content/dialects.md",
			spec:    "e2e/atago/v1_0_bugs.atago.yaml",
		},
		{
			name:    "the dialect a session is in",
			message: "current dialect: mysql (available: sqlite, mysql, postgresql, googlesql)",
			doc:     "website/content/dialects.md",
			spec:    "e2e/atago/dialect.atago.yaml",
		},
		{
			name:    "the confirmation that the dialect changed",
			message: "dialect set to mysql",
			doc:     "website/content/dialects.md",
			spec:    "e2e/atago/dialect.atago.yaml",
		},
		{
			name:    "the line that says where a result was written",
			message: "Output sql result to user.json (output mode=json)",
			doc:     "README.md",
			spec:    "e2e/atago/output_status.atago.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(readDoc(t, tt.doc), tt.message) {
				t.Errorf("%s no longer quotes the message the E2E pins:\n%s", tt.doc, tt.message)
			}
			if !strings.Contains(readDoc(t, tt.spec), tt.message) {
				t.Errorf("%s no longer asserts the message %s quotes:\n%s", tt.spec, tt.doc, tt.message)
			}
		})
	}
}
