package shell

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/nao1215/prompt"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

// newCompletionShell builds a shell whose session holds two tables, which is
// the smallest schema that can tell "a column of the table this statement
// names" from "a column that merely exists".
func newCompletionShell(t *testing.T) *Shell {
	t.Helper()

	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return([]*model.Table{
		model.NewTable("users", nil, nil),
		model.NewTable("orders", nil, nil),
	}, nil).AnyTimes()
	metadata.EXPECT().Header(gomock.Any(), "users").Return(
		model.NewTable("users", model.Header{"id", "name", "email"}, nil), nil).AnyTimes()
	metadata.EXPECT().Header(gomock.Any(), "orders").Return(
		model.NewTable("orders", model.Header{"id", "user_id", "total"}, nil), nil).AnyTimes()

	importer := mock.NewMockImportUsecase(ctrl)
	importer.EXPECT().IsSupportedFile(gomock.Any()).Return(false).AnyTimes()

	return newBoundaryTestShell(t, Usecases{metadata: metadata, importer: importer})
}

// TestCompletionAfterFromOffersTablesNotColumns covers the table position. A
// column name cannot be a table, and offering every column of every table after
// FROM buried the two names that could actually go there.
func TestCompletionAfterFromOffersTablesNotColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		first string
	}{
		{name: "FROM with nothing typed offers a table first", input: "SELECT * FROM ", first: "users"},
		{name: "a partial name after FROM offers the table it prefixes", input: "SELECT * FROM us", first: "users"},
		{name: "a partial name after JOIN offers the table it prefixes", input: "SELECT * FROM users JOIN or", first: "orders"},
		{name: "a partial name after INSERT INTO offers a table", input: "INSERT INTO us", first: "users"},
		{name: "a partial name after UPDATE offers a table", input: "UPDATE us", first: "users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newCompletionShell(t)
			got := completionTexts(s.getCompletions(context.Background(), tt.input))
			if len(got) == 0 || got[0] != tt.first {
				t.Fatalf("getCompletions(%q) first suggestion = %v, want %q", tt.input, got, tt.first)
			}
			// "user_id" is a column of orders; before the position was read it was
			// offered here because it prefix-matches "us".
			if slices.Contains(got, "user_id") {
				t.Errorf("getCompletions(%q) offered the column user_id in a table position: %v", tt.input, got)
			}
		})
	}
}

// TestCompletionResolvesQualifiedColumns covers the case that produced nothing
// at all: an alias or table name followed by a dot. The word is one word to the
// line editor, so filtering the flat candidate list by it matched nothing.
func TestCompletionResolvesQualifiedColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "an alias with a trailing dot offers that table's columns",
			input: "SELECT * FROM users u WHERE u.",
			want:  []string{"u.id", "u.name", "u.email"},
		},
		{
			name:  "a partial column after an alias is filtered by it",
			input: "SELECT * FROM users u WHERE u.na",
			want:  []string{"u.name"},
		},
		{
			name:  "an AS alias resolves the same way",
			input: "SELECT * FROM users AS u WHERE u.em",
			want:  []string{"u.email"},
		},
		{
			name:  "a join resolves each side to its own table",
			input: "SELECT * FROM users u JOIN orders o ON u.id = o.us",
			want:  []string{"o.user_id"},
		},
		{
			name:  "a table name qualifies its own columns",
			input: "SELECT * FROM orders WHERE orders.to",
			want:  []string{"orders.total"},
		},
		{
			name:  "a qualifier typed in another case still resolves",
			input: "SELECT * FROM users u WHERE U.NA",
			want:  []string{"U.name"},
		},
		{
			name:  "an alias for a table typed in another case still offers its columns",
			input: "SELECT * FROM USERS u WHERE u.em",
			want:  []string{"u.email"},
		},
		{
			name:  "an unknown qualifier offers nothing rather than every column",
			input: "SELECT * FROM users u WHERE nosuch.",
			want:  nil,
		},
		{
			name:  "a qualified name completes on a continuation line",
			input: "SELECT *\nFROM users u\nWHERE u.em",
			want:  []string{"u.email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newCompletionShell(t)
			got := completionTexts(s.getCompletions(context.Background(), tt.input))
			if !slices.Equal(got, tt.want) {
				t.Errorf("getCompletions(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestCompletionRanksColumnsOfTablesInScopeFirst covers the column position: a
// statement that already names its tables says which columns are worth
// offering first.
func TestCompletionRanksColumnsOfTablesInScopeFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		first string
	}{
		{name: "a WHERE over orders offers an orders column first", input: "SELECT * FROM orders WHERE ", first: "id"},
		{name: "a partial name in a WHERE over orders resolves to an orders column", input: "SELECT * FROM orders WHERE us", first: "user_id"},
		{name: "an ORDER BY over users resolves to a users column", input: "SELECT * FROM users ORDER BY na", first: "name"},
		{name: "an UPDATE ... SET resolves to that table's column", input: "UPDATE users SET em", first: "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newCompletionShell(t)
			got := completionTexts(s.getCompletions(context.Background(), tt.input))
			if len(got) == 0 || got[0] != tt.first {
				t.Errorf("getCompletions(%q) first suggestion = %v, want %q", tt.input, got, tt.first)
			}
		})
	}
}

// TestCompletionOnAFreshContinuationLine covers a line entered after Enter on
// an unfinished statement. The word being completed was taken as everything
// after the last space, which on a fresh line still held the newline, so it
// matched nothing and the menu was empty exactly where a long statement is
// being written.
func TestCompletionOnAFreshContinuationLine(t *testing.T) {
	t.Parallel()

	s := newCompletionShell(t)

	got := completionTexts(s.getCompletions(context.Background(), "SELECT name,\n"))
	if len(got) == 0 {
		t.Fatal("a fresh continuation line offered no completions at all")
	}
	if !slices.Contains(got, "email") {
		t.Errorf("a fresh continuation line did not offer a column: %v", got)
	}
}

// TestCompletionKeywordsAreCaseInsensitive covers the lower-cased word nobody
// types any other way. SQL keywords are offered upper-cased, so the match has
// to ignore case, and the suggestion has to say it replaces the typed word or
// the prompt appends it instead ("sel SELECT").
func TestCompletionKeywordsAreCaseInsensitive(t *testing.T) {
	t.Parallel()

	s := newCompletionShell(t)

	for _, input := range []string{"sel", "SEL", "Sel"} {
		got := completionTexts(s.getCompletions(context.Background(), input))
		if !slices.Contains(got, "SELECT") {
			t.Errorf("getCompletions(%q) = %v, want it to contain SELECT", input, got)
		}
	}
}

// TestCompletionReplacesTheWordBeingTyped covers what the prompt is told about
// each suggestion: the span it overwrites. Without it a case-insensitive match
// was dropped by the prompt's own filter, and a qualified name was appended
// beside the typed word rather than completing it.
func TestCompletionReplacesTheWordBeingTyped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		text      string
		wantStart int
	}{
		{name: "a partial keyword is replaced from where it starts", text: "sel", wantStart: 0},
		{name: "a qualified name is replaced whole", text: "SELECT * FROM users u WHERE u.na", wantStart: len("SELECT * FROM users u WHERE ")},
		{name: "an empty word after a space inserts at the cursor", text: "SELECT * FROM ", wantStart: len("SELECT * FROM ")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newCompletionShell(t)
			d := prompt.Document{Text: tt.text, CursorPosition: len([]rune(tt.text))}
			got := s.completeDocument(context.Background(), d)
			if len(got) == 0 {
				t.Fatalf("completeDocument(%q) returned no suggestions", tt.text)
			}
			for _, suggestion := range got {
				if suggestion.Replace == nil {
					t.Fatalf("suggestion %q carries no replacement span", suggestion.Text)
				}
				if suggestion.Replace.Start != tt.wantStart || suggestion.Replace.End != d.CursorPosition {
					t.Fatalf("suggestion %q replaces [%d,%d), want [%d,%d)",
						suggestion.Text, suggestion.Replace.Start, suggestion.Replace.End, tt.wantStart, d.CursorPosition)
				}
			}
		})
	}
}

// TestCompletionSpanIsCountedInRunes checks the span against a line holding
// multi-byte text. The prompt's cursor is a rune index, so a span measured in
// bytes would overwrite the wrong part of the line.
func TestCompletionSpanIsCountedInRunes(t *testing.T) {
	t.Parallel()

	s := newCompletionShell(t)

	const text = "SELECT '日本語' FROM us"
	d := prompt.Document{Text: text, CursorPosition: len([]rune(text))}

	got := s.completeDocument(context.Background(), d)
	if len(got) == 0 {
		t.Fatal("no suggestions for a line containing multi-byte text")
	}
	wantStart := len([]rune(text)) - len("us")
	for _, suggestion := range got {
		if suggestion.Replace.Start != wantStart {
			t.Fatalf("suggestion %q replaces from %d, want %d", suggestion.Text, suggestion.Replace.Start, wantStart)
		}
	}
}

// TestCompletionSuggestionsAreUnique checks that no name is offered twice. A
// table and a column can share a name, and the ordered groups overlap, so
// without a check the same text appears in two places in one menu.
func TestCompletionSuggestionsAreUnique(t *testing.T) {
	t.Parallel()

	s := newCompletionShell(t)

	inputs := []string{
		"", "SELECT ", "SELECT * FROM ", "SELECT * FROM users WHERE ",
		"SELECT * FROM users u JOIN orders o ON ", "UPDATE users SET ", "i",
	}
	for _, input := range inputs {
		seen := map[string]bool{}
		for _, text := range completionTexts(s.getCompletions(context.Background(), input)) {
			if seen[text] {
				t.Errorf("getCompletions(%q) offered %q twice", input, text)
			}
			seen[text] = true
		}
	}
}

// TestCompletionEveryCandidateMatchesTheTypedWord is a property over the
// candidate list: whatever the statement says about the cursor, a suggestion
// the user cannot reach by continuing to type is noise in the menu.
func TestCompletionEveryCandidateMatchesTheTypedWord(t *testing.T) {
	t.Parallel()

	s := newCompletionShell(t)

	inputs := []string{
		"s", "se", "sel", "SELECT i", "SELECT * FROM u", "SELECT * FROM us",
		"SELECT * FROM users WHERE e", "SELECT * FROM users u WHERE u.n",
		"SELECT * FROM users ORDER BY na", "UPDATE users SET em", ".he", ".mod",
	}
	for _, input := range inputs {
		word := currentCompletionWord(input)
		for _, text := range completionTexts(s.getCompletions(context.Background(), input)) {
			if !strings.HasPrefix(strings.ToLower(text), strings.ToLower(word)) {
				t.Errorf("getCompletions(%q) offered %q, which the typed word %q does not prefix", input, text, word)
			}
		}
	}
}

// TestCompletionSeesAColumnAddedBySQL covers the cache after a statement that
// changes a table's shape. The cache is keyed by the table-name set, which an
// ALTER TABLE ... ADD COLUMN leaves untouched, so the columns kept were the
// ones from before the statement ran and the new one could not be completed.
func TestCompletionSeesAColumnAddedBySQL(t *testing.T) {
	// Serial: newShell builds an in-memory DB and a temp history path.
	s, cleanup, err := newShell(t, []string{"sqly"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	ctx := context.Background()
	if err := s.exec(ctx, "CREATE TABLE t (a)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	// Warm the cache so the ALTER below has a stale entry to invalidate.
	if got := completionTexts(s.getCompletions(ctx, "SELECT a")); !slices.Contains(got, "a") {
		t.Fatalf("the column a was not completed before the ALTER: %v", got)
	}

	if err := s.exec(ctx, "ALTER TABLE t ADD COLUMN newcol"); err != nil {
		t.Fatalf("ALTER TABLE: %v", err)
	}

	got := completionTexts(s.getCompletions(ctx, "SELECT newc"))
	if !slices.Contains(got, "newcol") {
		t.Errorf("a column added by ALTER TABLE is not completed: %v", got)
	}
}
