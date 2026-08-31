package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/config"
)

// TestSplitPathPrefix_QuotedBranches exercises the shapes splitPathPrefix can
// return for a quoted path: no separator, a normal directory prefix, a leading
// separator, and a Windows-style separator. readDir is asserted after
// toScanPath, which is what the caller reads the filesystem with — the split
// returns the directory as typed, and turning it into a real path is a separate
// step because what an escape means depends on how the path was written.
func TestSplitPathPrefix_QuotedBranches(t *testing.T) {
	t.Parallel()

	root := string(os.PathSeparator)
	tests := []struct {
		name        string
		in          string
		wantReadDir string
		wantBase    string
		wantPartial string
	}{
		{
			name:        "no separator -> scan current dir, empty base",
			in:          "sample",
			wantReadDir: ".",
			wantBase:    "",
			wantPartial: "sample",
		},
		{
			name:        "dir prefix -> scan that dir, keep base, match partial",
			in:          "testdata/sa",
			wantReadDir: "testdata" + string(os.PathSeparator),
			wantBase:    "testdata/",
			wantPartial: "sa",
		},
		{
			name:        "leading separator -> base is the separator itself",
			in:          "/sample",
			wantReadDir: root,
			wantBase:    "/",
			wantPartial: "sample",
		},
		{
			name:        "backslash separator is normalized for readDir",
			in:          `dir\sa`,
			wantReadDir: "dir" + string(os.PathSeparator),
			wantBase:    `dir\`,
			wantPartial: "sa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			readDir, base, partial := splitPathPrefix(tt.in, lastAnySeparator)
			if got := toScanPath(readDir); got != tt.wantReadDir || base != tt.wantBase || partial != tt.wantPartial {
				t.Errorf("splitPathPrefix(%q, lastAnySeparator) = (%q, %q, %q), want (%q, %q, %q)",
					tt.in, got, base, partial, tt.wantReadDir, tt.wantBase, tt.wantPartial)
			}
		})
	}
}

// TestDecodeQuotedPath_QuoteHandling checks that single-quoted content stays
// literal while double-quoted content unescapes \" and \\.
func TestDecodeQuotedPath_QuoteHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		quote rune
		want  string
	}{
		{
			name:  "single quote keeps content literal",
			in:    `a\"b\\c`,
			quote: '\'',
			want:  `a\"b\\c`,
		},
		{
			name:  "double quote unescapes escaped quote and backslash",
			in:    `a\"b\\c`,
			quote: '"',
			want:  `a"b\c`,
		},
		{
			name:  "double quote leaves a non-escaping backslash intact",
			in:    `a\b`,
			quote: '"',
			want:  `a\b`,
		},
		{
			name:  "double quote trailing backslash stays literal",
			in:    `ab\`,
			quote: '"',
			want:  `ab\`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := decodeQuotedPath(tt.in, tt.quote); got != tt.want {
				t.Errorf("decodeQuotedPath(%q, %q) = %q, want %q", tt.in, tt.quote, got, tt.want)
			}
		})
	}
}

// TestOpenQuotePrefix_Branches covers every classification openQuotePrefix makes:
// empty word, non-quote leading rune, a closed quote, an escaped quote inside a
// double quote that does not close it, and a still-open quote.
func TestOpenQuotePrefix_Branches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		word      string
		wantQuote rune
		wantInner string
		wantOK    bool
	}{
		{name: "empty word is not a quoted prefix", word: "", wantQuote: 0, wantInner: "", wantOK: false},
		{name: "non-quote leading rune is not a quoted prefix", word: "abc", wantQuote: 0, wantInner: "", wantOK: false},
		{name: "single-quoted but already closed is not in-progress", word: "'done'", wantQuote: 0, wantInner: "", wantOK: false},
		{name: "open single quote returns inner text", word: "'my dir", wantQuote: '\'', wantInner: "my dir", wantOK: true},
		{name: "open double quote returns inner text", word: `"my dir`, wantQuote: '"', wantInner: "my dir", wantOK: true},
		{name: "double quote with escaped quote stays open", word: `"a\"b`, wantQuote: '"', wantInner: `a\"b`, wantOK: true},
		{name: "double quote closed after escaped quote is not in-progress", word: `"a\"b"`, wantQuote: 0, wantInner: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			quote, inner, ok := openQuotePrefix(tt.word)
			if quote != tt.wantQuote || inner != tt.wantInner || ok != tt.wantOK {
				t.Errorf("openQuotePrefix(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.word, quote, inner, ok, tt.wantQuote, tt.wantInner, tt.wantOK)
			}
		})
	}
}

// TestSplitPathPrefix_LeadingSeparator covers a path rooted at "/". base always
// holds at least the separator, so a rooted path needs no special case.
func TestSplitPathPrefix_LeadingSeparator(t *testing.T) {
	t.Parallel()

	// lastUnescapedSeparator treats "/" as a path separator on every platform, so
	// the split is "/" + "foo" and readDir mirrors the non-empty base "/". This is
	// independent of os.PathSeparator, which is "\\" on Windows.
	readDir, base, partial := splitPathPrefix("/foo", lastUnescapedSeparator)
	if readDir != "/" || base != "/" || partial != "foo" {
		t.Errorf("splitPathPrefix(%q, lastUnescapedSeparator) = (%q, %q, %q), want (%q, %q, %q)",
			"/foo", readDir, base, partial, "/", "/", "foo")
	}
}

// TestCompletedCommandWords_FallbackOnBadQuote covers the fallback path where the
// already-typed prefix cannot be tokenized (an unterminated quote) and the code
// degrades to whitespace splitting instead of returning nothing.
func TestCompletedCommandWords_FallbackOnBadQuote(t *testing.T) {
	t.Parallel()

	// prefix becomes `.import "bad `, which splitArgs cannot tokenize.
	got := completedCommandWords(`.import "bad file`, "file")
	want := []string{".import", `"bad`}
	if len(got) != len(want) {
		t.Fatalf("completedCommandWords fallback = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("completedCommandWords fallback = %#v, want %#v", got, want)
		}
	}
}

// TestExpandTilde_NoHomeReturnsError covers the os.UserHomeDir failure branch by
// clearing HOME so a tilde path cannot be expanded.
func TestExpandTilde_NoHomeReturnsError(t *testing.T) {
	// Serial: mutates the HOME environment variable via t.Setenv.
	if runtime.GOOS == "windows" {
		t.Skip("clearing HOME does not fail os.UserHomeDir on Windows")
	}
	t.Setenv("HOME", "")

	if _, err := expandTilde("~"); err == nil {
		t.Fatal("expandTilde(\"~\") with empty HOME should return an error")
	}
	// A path without a tilde prefix short-circuits before the home lookup.
	if got, err := expandTilde("plain/path"); err != nil || got != "plain/path" {
		t.Fatalf("expandTilde(non-tilde) = (%q, %v), want (%q, nil)", got, err, "plain/path")
	}
}

// TestShortCWD_NoHomeReturnsCWD covers shortCWD's fallback: when the home
// directory cannot be resolved, the raw working directory is returned unchanged.
func TestShortCWD_NoHomeReturnsCWD(t *testing.T) {
	// Serial: mutates the HOME environment variable via t.Setenv.
	if runtime.GOOS == "windows" {
		t.Skip("clearing HOME does not fail os.UserHomeDir on Windows")
	}
	t.Setenv("HOME", "")

	st := &state{cwd: "/some/where"}
	if got := st.shortCWD(); got != "/some/where" {
		t.Fatalf("shortCWD() = %q, want unchanged cwd", got)
	}
}

// TestPwdCommand_RejectsArguments covers the argument-count guard of .pwd.
func TestPwdCommand_RejectsArguments(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	err = shell.commands.pwdCommand(context.Background(), shell, []string{"unexpected"})
	if err == nil || !strings.Contains(err.Error(), ".pwd takes no arguments") {
		t.Fatalf(".pwd with args error = %v, want argument-count error", err)
	}
}

// TestTableColumns_SchemaQualifiedName covers the schema-qualified branch of
// tableColumns, where a "main.<table>" argument is inspected against that schema.
func TestTableColumns_SchemaQualifiedName(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if err := shell.commands.importCommand(context.Background(), shell, []string{"testdata/user.csv"}); err != nil {
		t.Fatal(err)
	}

	cols, err := shell.tableColumns(context.Background(), "main.user")
	if err != nil {
		t.Fatalf("tableColumns(main.user) error: %v", err)
	}
	if len(cols.Records()) == 0 {
		t.Fatal("tableColumns(main.user) returned no columns, want the user table columns")
	}
	if cols.Name() != "main.user" {
		t.Fatalf("tableColumns name = %q, want %q", cols.Name(), "main.user")
	}
}

// TestExec_UnknownDotCommand covers the exec branch that reports an unknown
// helper command instead of trying to run it as SQL.
func TestExec_UnknownDotCommand(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	err = shell.exec(context.Background(), ".definitely-not-a-command")
	if err == nil || !strings.Contains(err.Error(), "no such sqly command") {
		t.Fatalf(".exec unknown dot command error = %v, want no-such-command error", err)
	}
}

// TestExec_HeaderIsNoLongerACommand pins that .header is gone rather than
// quietly doing something else. It printed the column names .describe prints in
// its name column, and answering one question two ways is a choice a reader has
// to make for nothing.
func TestExec_HeaderIsNoLongerACommand(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	err = shell.exec(context.Background(), ".header user")
	if err == nil || !strings.Contains(err.Error(), "no such sqly command") {
		t.Fatalf(".header error = %v, want no-such-command error", err)
	}
}

// TestCommunicate_PromptSessionCreationFails covers the error path where the
// prompt session cannot be created and communicate wraps it with guidance.
func TestCommunicate_PromptSessionCreationFails(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.historyEnabled = false
	backendErr := errors.New("open /dev/tty: no such device")
	shell.newPrompt = func(string, func(prompt.Document) []prompt.Suggestion) (promptSession, error) {
		return nil, backendErr
	}

	err = shell.communicate(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cannot start the interactive shell") {
		t.Fatalf("communicate error = %v, want a clear no-terminal message", err)
	}
	if !errors.Is(err, backendErr) {
		t.Fatalf("communicate error = %v, want to wrap the backend error", err)
	}
}

// TestStageStdinDataset_UnsupportedFormat covers the guard that rejects an
// unknown --stdin-format value before touching the filesystem.
func TestStageStdinDataset_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.argument.StdinFormat = "xml"
	shell.argument.StdinTableName = "stdin"

	_, _, err = shell.stageStdinDataset(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported --stdin-format value") {
		t.Fatalf("stageStdinDataset unsupported format error = %v, want format error", err)
	}
}

// TestStageStdinDataset_SuccessStagesFile covers the success path: stdin is read
// into a temp file named after the stdin table, and the cleanup removes it.
func TestStageStdinDataset_SuccessStagesFile(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	shell.argument.StdinFormat = "csv"
	shell.argument.StdinTableName = "piped"
	shell.stdin = strings.NewReader("a,b\n1,2\n")

	path, stageCleanup, err := shell.stageStdinDataset(context.Background())
	if err != nil {
		t.Fatalf("stageStdinDataset error: %v", err)
	}
	defer stageCleanup()

	if filepath.Base(path) != "piped.csv" {
		t.Fatalf("staged file = %q, want a piped.csv name", path)
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is a sqly-generated temp path
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(data) != "a,b\n1,2\n" {
		t.Fatalf("staged content = %q, want the piped bytes", string(data))
	}

	stageCleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup did not remove staged file, stat err = %v", err)
	}
}

// TestOutputAliasesImportedSource_SkipsStdinAndMatchesFile covers both the skip
// of stdin-backed tables and the positive match of a real source file.
func TestOutputAliasesImportedSource_SkipsStdinAndMatchesFile(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	src, err := filepath.Abs("testdata/user.csv")
	if err != nil {
		t.Fatal(err)
	}
	shell.tableSources = map[string]string{
		"stdintable": stdinTableSource,
		"user":       src,
	}

	t.Run("real source file is reported as an alias", func(t *testing.T) {
		name, aliased := shell.outputAliasesImportedSource(src)
		if !aliased || name != "user" {
			t.Fatalf("outputAliasesImportedSource(%q) = (%q, %v), want (user, true)", src, name, aliased)
		}
	})

	t.Run("unrelated destination is not an alias", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "out.csv")
		if name, aliased := shell.outputAliasesImportedSource(other); aliased {
			t.Fatalf("outputAliasesImportedSource(%q) = (%q, true), want no alias", other, name)
		}
	})
}

// TestRunSQLFileToOutput_ResultSetCount covers the zero-result and multiple-result
// error branches of the --sql-file --output contract.
// Serial: the "one result set" subtest swaps the package-level config.Stderr that
// runSQLFileToOutput writes to, so the subtests must not run concurrently.
func TestRunSQLFileToOutput_ResultSetCount(t *testing.T) {
	t.Run("no result set -> error asks for a SELECT", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.argument.Output.FilePath = filepath.Join(t.TempDir(), "out.csv")

		err = shell.runSQLFileToOutput(context.Background(), mustParse(t, "CREATE TABLE t (a INTEGER);"))
		if err == nil || !strings.Contains(err.Error(), "produced none") {
			t.Fatalf("runSQLFileToOutput no-result error = %v, want none-produced error", err)
		}
	})

	t.Run("one result set -> exports to the output file", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		out := filepath.Join(t.TempDir(), "out.csv")
		shell.argument.Output.FilePath = out

		// Status goes to config.Stderr; capture it so it does not leak to the test log.
		backupStderr := config.Stderr
		defer func() { config.Stderr = backupStderr }()
		config.Stderr = &strings.Builder{}

		if err := shell.runSQLFileToOutput(context.Background(), mustParse(t, "SELECT 1 AS a;")); err != nil {
			t.Fatalf("runSQLFileToOutput single-result error: %v", err)
		}
		data, err := os.ReadFile(out) //nolint:gosec // out is a test temp path
		if err != nil {
			t.Fatalf("read exported file: %v", err)
		}
		if !strings.Contains(string(data), "a") {
			t.Fatalf("exported file = %q, want the SELECT result", string(data))
		}
	})

	t.Run("two result sets -> error asks to reduce to one", func(t *testing.T) {
		shell, cleanup, err := newShell(t, []string{"sqly"})
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		shell.argument.Output.FilePath = filepath.Join(t.TempDir(), "out.csv")

		err = shell.runSQLFileToOutput(context.Background(), mustParse(t, "SELECT 1 AS a;\nSELECT 2 AS b;"))
		if err == nil || !strings.Contains(err.Error(), "result sets") {
			t.Fatalf("runSQLFileToOutput two-result error = %v, want single-result error", err)
		}
		// --output-format is not a way out here, so the message must not offer it.
		if strings.Contains(err.Error(), "--output-format table") {
			t.Errorf("error advises a format that also fails with --output: %v", err)
		}
		if !strings.Contains(err.Error(), "--output") {
			t.Errorf("error does not name the flag that imposes the limit: %v", err)
		}
	})
}

// TestGetQuotedFilePathCompletions_DirAndBadDir covers a directory suggestion
// inside an open quote and the ReadDir failure that yields no suggestions.
func TestGetQuotedFilePathCompletions_DirAndBadDir(t *testing.T) {
	t.Parallel()

	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	t.Run("open quote inside testdata offers entries", func(t *testing.T) {
		got := shell.getQuotedFilePathCompletions("testdata/", '"')
		if len(got) == 0 {
			t.Fatal("getQuotedFilePathCompletions(testdata/) returned nothing, want entries")
		}
	})

	t.Run("non-existent directory yields no suggestions", func(t *testing.T) {
		got := shell.getQuotedFilePathCompletions("no_such_dir_here/", '"')
		if len(got) != 0 {
			t.Fatalf("getQuotedFilePathCompletions(bad dir) = %#v, want none", got)
		}
	})

	// The quoted and escaped paths differ in how a suggestion is written, never in
	// which entries are offered. They read one directory through one function, so
	// the two lists must name the same entries in the same order; a completion
	// rule added to one and not the other is what this catches.
	t.Run("quoted and escaped completion offer the same entries", func(t *testing.T) {
		quoted := shell.getQuotedFilePathCompletions("testdata/", '"')
		escaped := shell.getFilePathCompletions("testdata/")
		if len(quoted) != len(escaped) {
			t.Fatalf("quoted offered %d entries, escaped offered %d", len(quoted), len(escaped))
		}
		for i := range quoted {
			// Strip the rendering each style adds: the opening quote, and the
			// closing one a file gets.
			bare := strings.TrimSuffix(strings.TrimPrefix(quoted[i].Text, `"`), `"`)
			if bare != escaped[i].Text {
				t.Errorf("entry %d: quoted %q, escaped %q; the two must offer the same entries",
					i, quoted[i].Text, escaped[i].Text)
			}
			if quoted[i].Description != escaped[i].Description {
				t.Errorf("entry %d: quoted describes %q, escaped describes %q",
					i, quoted[i].Description, escaped[i].Description)
			}
		}
	})

	// Hidden entries are skipped unless the user types a leading dot, in both
	// styles. This is the rule the two copies of the scan each carried.
	t.Run("a leading dot reveals hidden entries in both styles", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".hidden.csv"), []byte("a\n1\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "shown.csv"), []byte("a\n1\n"), 0o600); err != nil {
			t.Fatal(err)
		}

		for _, tc := range []struct {
			style string
			all   []Suggest
			dot   []Suggest
		}{
			{"escaped", shell.getFilePathCompletions(dir + "/"), shell.getFilePathCompletions(dir + "/.")},
			{"quoted", shell.getQuotedFilePathCompletions(dir+"/", '"'), shell.getQuotedFilePathCompletions(dir+"/.", '"')},
		} {
			if len(tc.all) != 1 {
				t.Errorf("%s: offered %d entries without a leading dot, want only the visible one", tc.style, len(tc.all))
			}
			if len(tc.dot) != 1 {
				t.Errorf("%s: offered %d entries after a leading dot, want only the hidden one", tc.style, len(tc.dot))
			}
			if len(tc.dot) == 1 && !strings.Contains(tc.dot[0].Text, ".hidden.csv") {
				t.Errorf("%s: a leading dot offered %q, want the hidden entry", tc.style, tc.dot[0].Text)
			}
		}
	})
}

// TestGetCompletionsDoesNotOfferAFileInATablePosition covers what a SQL
// statement can name. A bare word after FROM used to also be completed as a
// filename in the working directory, but no SQL statement in sqly takes a path:
// a file becomes a table through .import or the command line, and the engine
// rejects the path itself ("no such table: user.csv"). Worse, the file a table
// was read from shares its prefix, so the invalid candidate sat beside the
// valid one and neither completed.
func TestGetCompletionsDoesNotOfferAFileInATablePosition(t *testing.T) {
	// Serial: uses t.Chdir so the importable files resolve from ".".
	shell, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	t.Chdir("testdata")

	for _, s := range shell.getCompletions(context.Background(), "SELECT * FROM us") {
		if strings.HasSuffix(s.Text, ".csv") {
			t.Fatalf("getCompletions offered the file %q where only a table can go", s.Text)
		}
	}
}
