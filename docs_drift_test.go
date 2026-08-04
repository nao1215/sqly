package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/shell"
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
	end := len(rest)
	for level := 1; level <= depth; level++ {
		marker := "\n" + strings.Repeat("#", level) + " "
		if at := strings.Index(rest, marker); at >= 0 && at < end {
			end = at
		}
	}
	return rest[:end]
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
	for _, file := range []string{"report.sql", "update.sqly", "sales.csv"} {
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
	for _, want := range []string{
		"none of the files in that import are committed",
		"table-name collision",
		"in the order you wrote them",
	} {
		if !strings.Contains(recipe, want) {
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
