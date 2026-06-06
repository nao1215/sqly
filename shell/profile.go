package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// profileColumn is the data-quality summary for one column.
type profileColumn struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`
	NullCount     int64    `json:"null_count"`
	BlankCount    int64    `json:"blank_count"`
	DistinctCount int64    `json:"distinct_count"`
	NumericCount  int64    `json:"numeric_count"`
	Warnings      []string `json:"warnings"`
}

// profileTable is the data-quality summary for one imported table.
type profileTable struct {
	Name        string          `json:"name"`
	Source      string          `json:"source,omitempty"`
	RowCount    int64           `json:"row_count"`
	ColumnCount int             `json:"column_count"`
	Columns     []profileColumn `json:"columns"`
}

// profileReport is the top-level JSON contract produced by --profile.
type profileReport struct {
	Tables []profileTable `json:"tables"`
}

// validateProfileFlags rejects --profile combined with flags that ask for a
// different action or side effect, mirroring --inspect and --compare.
func (s *Shell) validateProfileFlags() error {
	if !s.argument.ProfileFlag {
		return nil
	}
	switch {
	case s.argument.InspectFlag:
		return errProfileConflict("--inspect")
	case s.argument.CompareFlag:
		return errProfileConflict("--compare")
	case s.argument.Query != "":
		return errProfileConflict("--sql")
	case s.argument.SQLFilePath != "":
		return errProfileConflict("--sql-file")
	case s.argument.Output != nil && s.argument.Output.FilePath != "":
		return errProfileConflict("--output")
	case s.argument.SaveInPlace:
		return errProfileConflict("--save")
	case s.argument.SaveDir != "":
		return errProfileConflict("--save-dir")
	case s.argument.Output != nil && s.argument.Output.Mode != model.PrintModeTable:
		return fmt.Errorf("--profile cannot be combined with an output mode flag (--%s)", s.argument.Output.Mode.String())
	}
	return nil
}

func errProfileConflict(flag string) error {
	return fmt.Errorf("--profile cannot be combined with %s", flag)
}

// runProfile prints a data-quality report for every imported table: row and
// column counts, per-column null/blank/distinct/numeric counts, and warnings for
// mixed-type columns, null-like placeholder text, and surrounding whitespace.
// JSON is the default automation contract; --profile-format text prints a
// human-readable summary. It is the non-interactive discovery path for users who
// received unfamiliar data and want to understand it before writing SQL.
func (s *Shell) runProfile(ctx context.Context) error {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		return errors.New("no tables to profile: provide input files or directories")
	}

	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	sort.Strings(names) // deterministic regardless of import order

	report := profileReport{Tables: make([]profileTable, 0, len(names))}
	for _, name := range names {
		entry, err := s.profileTable(ctx, name)
		if err != nil {
			return err
		}
		report.Tables = append(report.Tables, entry)
	}

	if s.argument.ProfileFormat == outputFormatText {
		fmt.Fprint(config.Stdout, renderProfileText(report))
		return nil
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode profile report: %w", err)
	}
	fmt.Fprintln(config.Stdout, string(encoded))
	return nil
}

// profileTable builds the data-quality summary for a single table by scanning its
// rows once and computing per-column statistics.
func (s *Shell) profileTable(ctx context.Context, name string) (profileTable, error) {
	columns, err := s.inspectColumns(ctx, name)
	if err != nil {
		return profileTable{}, err
	}
	table, err := s.selectAll(ctx, name)
	if err != nil {
		return profileTable{}, err
	}

	typeByName := make(map[string]string, len(columns))
	for _, c := range columns {
		typeByName[c.Name] = c.Type
	}

	header := table.Header()
	records := table.Records()
	cols := make([]profileColumn, 0, len(header))
	for ci, colName := range header {
		values := make([]string, len(records))
		nulls := make([]bool, len(records))
		for ri, rec := range records {
			if ci < len(rec) {
				values[ri] = rec[ci]
			}
			nulls[ri] = table.IsNull(ri, ci)
		}
		cols = append(cols, profileColumnStats(colName, typeByName[colName], values, nulls))
	}

	return profileTable{
		Name:        name,
		Source:      s.tableSources[name],
		RowCount:    int64(len(records)),
		ColumnCount: len(header),
		Columns:     cols,
	}, nil
}

// nullLikeTokens are non-empty strings that often stand in for a missing value.
// A column that contains them is flagged so the user can decide whether to treat
// them as real NULLs before querying.
var nullLikeTokens = map[string]struct{}{
	"null": {}, "n/a": {}, "na": {}, "nil": {}, "none": {}, "#n/a": {}, "nan": {},
}

// profileColumnStats computes the data-quality summary for one column from its
// values and NULL mask. It is pure so it can be unit-tested directly. Warnings
// are only raised where they can be inferred safely: a mix of numeric and
// non-numeric non-empty values, null-like placeholder text, and values with
// leading or trailing whitespace.
func profileColumnStats(name, typ string, values []string, nulls []bool) profileColumn {
	pc := profileColumn{Name: name, Type: typ, Warnings: []string{}}
	distinct := make(map[string]struct{})
	var numeric, nonNumeric, nullLike, whitespace int64
	for i, v := range values {
		if i < len(nulls) && nulls[i] {
			pc.NullCount++
			continue
		}
		if v == "" {
			pc.BlankCount++
			continue
		}
		distinct[v] = struct{}{}
		if isNumericValue(v) {
			numeric++
		} else {
			nonNumeric++
		}
		if _, ok := nullLikeTokens[strings.ToLower(v)]; ok {
			nullLike++
		}
		if v != strings.TrimSpace(v) {
			whitespace++
		}
	}
	pc.DistinctCount = int64(len(distinct))
	pc.NumericCount = numeric

	if numeric > 0 && nonNumeric > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("mixed numeric and non-numeric values (%d numeric, %d non-numeric)", numeric, nonNumeric))
	}
	if nullLike > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("%d value(s) look like null placeholders (e.g. NULL, N/A)", nullLike))
	}
	if whitespace > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("%d value(s) have leading or trailing whitespace", whitespace))
	}
	return pc
}

// isNumericValue reports whether s is a finite decimal number. Infinity and NaN
// spellings are excluded so a column of the literal text "NaN" is not counted as
// numeric.
func isNumericValue(s string) bool {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false
	}
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

// renderProfileText renders a human-readable summary of a profile report.
func renderProfileText(r profileReport) string {
	var b strings.Builder
	for ti, t := range r.Tables {
		if ti > 0 {
			fmt.Fprintln(&b)
		}
		fmt.Fprintf(&b, "table %s: %d rows, %d columns\n", t.Name, t.RowCount, t.ColumnCount)
		for _, c := range t.Columns {
			fmt.Fprintf(&b, "  %s (%s): nulls=%d blanks=%d distinct=%d numeric=%d\n",
				c.Name, c.Type, c.NullCount, c.BlankCount, c.DistinctCount, c.NumericCount)
			for _, w := range c.Warnings {
				fmt.Fprintf(&b, "    warning: %s\n", w)
			}
		}
	}
	return b.String()
}
