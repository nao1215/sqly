package shell

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/interactor/mock"
	"go.uber.org/mock/gomock"
)

// TestMissingName pins what is read out of SQLite's message. The name is
// followed by the statement that failed, so anything that took the rest of the
// line would quote a whole query back at the reader.
func TestMissingName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		msg    string
		marker string
		want   string
		wantOK bool
	}{
		{
			name:   "a table name is read up to the detail that follows it",
			msg:    "execute query error: SQL logic error: no such table: staf (1): SELECT * FROM staf",
			marker: "no such table: ",
			want:   "staf",
			wantOK: true,
		},
		{
			name:   "a column name is read the same way",
			msg:    "execute query error: SQL logic error: no such column: nam (1): SELECT nam FROM staff",
			marker: "no such column: ",
			want:   "nam",
			wantOK: true,
		},
		{
			name:   "a name at the very end of the message is still read",
			msg:    "no such table: staf",
			marker: "no such table: ",
			want:   "staf",
			wantOK: true,
		},
		{
			name:   "a marker that is not there reads nothing",
			msg:    "database is locked",
			marker: "no such table: ",
			wantOK: false,
		},
		{
			name:   "a marker with nothing after it reads nothing",
			msg:    "no such table: ",
			marker: "no such table: ",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := missingName(tt.msg, tt.marker)
			if ok != tt.wantOK {
				t.Fatalf("missingName(%q) ok = %v, want %v", tt.msg, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("missingName(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

// TestMissingTableHint covers what a session with too many tables to list says.
// A workbook of a hundred sheets is a hundred tables, and a hint that prints all
// of them is not a hint; the count says how much was left out.
func TestMissingTableHint(t *testing.T) {
	tests := []struct {
		name       string
		tables     int
		wantParts  []string
		wantAbsent []string
	}{
		{
			name:      "no tables at all says how to get one",
			tables:    0,
			wantParts: []string{"this session has no tables", ".import"},
			// There is nothing to list, and the naming rules explain a name that
			// exists rather than the absence of every name.
			wantAbsent: []string{"Available tables", tableNameRulesURL},
		},
		{
			name:       "a few tables are all listed",
			tables:     3,
			wantParts:  []string{`no table "staf"`, "Available tables: t01, t02, t03.", tableNameRulesURL},
			wantAbsent: []string{"total)"},
		},
		{
			name:       "the cap is not reached at exactly twenty",
			tables:     maxHintedTables,
			wantParts:  []string{"t20."},
			wantAbsent: []string{"total)"},
		},
		{
			name:       "one over the cap lists twenty and counts the rest",
			tables:     maxHintedTables + 1,
			wantParts:  []string{"t20", "... (21 total)"},
			wantAbsent: []string{"t21,"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			metadata := mock.NewMockMetadataUsecase(ctrl)
			tables := make([]*model.Table, 0, tt.tables)
			for i := 1; i <= tt.tables; i++ {
				tables = append(tables, model.NewTable(fmt.Sprintf("t%02d", i), model.NewHeader([]string{"n"}), nil))
			}
			metadata.EXPECT().TablesName(gomock.Any()).Return(tables, nil)

			s := newBoundaryTestShell(t, Usecases{metadata: metadata})
			got, ok := s.missingTableHint(context.Background(), "staf")
			if !ok {
				t.Fatal("missingTableHint reported no hint for a readable table list")
			}
			for _, want := range tt.wantParts {
				if !strings.Contains(got, want) {
					t.Errorf("hint %q does not contain %q", got, want)
				}
			}
			for _, unwanted := range tt.wantAbsent {
				if strings.Contains(got, unwanted) {
					t.Errorf("hint %q contains %q", got, unwanted)
				}
			}
		})
	}
}

// TestMissingTableHint_StaysQuietWhenTheListCannotBeRead is the case where
// saying nothing is right. A session that holds tables and one whose list failed
// would otherwise both be described as empty, and telling someone to pass a file
// they already passed is worse than no hint.
func TestMissingTableHint_StaysQuietWhenTheListCannotBeRead(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)
	metadata.EXPECT().TablesName(gomock.Any()).Return(nil, errors.New("database is locked"))

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	if _, ok := s.missingTableHint(context.Background(), "staf"); ok {
		t.Error("missingTableHint offered a hint built from a table list it could not read")
	}
}

// TestWithMissingNameHint_LeavesOtherErrorsAlone keeps the hint to the two
// failures it is about. Every other statement error is reported as it came.
func TestWithMissingNameHint_LeavesOtherErrorsAlone(t *testing.T) {
	ctrl := gomock.NewController(t)
	metadata := mock.NewMockMetadataUsecase(ctrl)

	s := newBoundaryTestShell(t, Usecases{metadata: metadata})
	err := errors.New("execute query error: SQL logic error: near \"SELCT\": syntax error")
	if got := s.withMissingNameHint(context.Background(), err); got.Error() != err.Error() {
		t.Errorf("withMissingNameHint rewrote an unrelated error: %v", got)
	}
}
