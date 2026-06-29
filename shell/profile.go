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
		return fmt.Errorf("--profile cannot be combined with an output mode flag (--%s)", outputModeFlagName(s.argument.Output))
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

	// Aggregate per column in a single pass over the rows. Why not build a
	// per-column values/nulls slice first: that duplicated the whole table once
	// per column. The accumulators hold only running counts (and the distinct set
	// the report needs anyway), so profiling no longer scales memory with
	// columns * rows.
	profilers := make([]*columnProfiler, len(header))
	for ci, colName := range header {
		profilers[ci] = newColumnProfiler(colName, typeByName[colName])
	}
	for ri, rec := range records {
		for ci := range header {
			var v string
			if ci < len(rec) {
				v = rec[ci]
			}
			profilers[ci].add(v, table.IsNull(ri, ci))
		}
	}
	cols := make([]profileColumn, len(header))
	for ci := range header {
		cols[ci] = profilers[ci].result()
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

// columnProfiler accumulates a column's data-quality statistics one value at a
// time, so profiling can stream rows without copying each column into its own
// full-length buffer. The distinct set is the only per-value memory it keeps,
// which the distinct count requires regardless.
type columnProfiler struct {
	name, typ                                                      string
	distinct                                                       map[string]struct{}
	nullCount, blankCount, numeric, nonNumeric, nullLike, spacecnt int64
}

// newColumnProfiler returns a profiler for a single named, typed column.
func newColumnProfiler(name, typ string) *columnProfiler {
	return &columnProfiler{name: name, typ: typ, distinct: make(map[string]struct{})}
}

// add folds one cell value (and whether it is SQL NULL) into the running stats.
func (c *columnProfiler) add(v string, isNull bool) {
	if isNull {
		c.nullCount++
		return
	}
	if v == "" {
		c.blankCount++
		// The blank string is a real distinct value. Counting it keeps
		// distinct_count consistent with blank_count so the report does not
		// understate cardinality for categorical columns that mix blanks with
		// real values.
		c.distinct[v] = struct{}{}
		return
	}
	c.distinct[v] = struct{}{}
	if isNumericValue(v) {
		c.numeric++
	} else {
		c.nonNumeric++
	}
	if _, ok := nullLikeTokens[strings.ToLower(v)]; ok {
		c.nullLike++
	}
	if v != strings.TrimSpace(v) {
		c.spacecnt++
	}
}

// result finalizes the accumulated statistics into a profileColumn, raising the
// same warnings as a full-buffer pass would.
func (c *columnProfiler) result() profileColumn {
	pc := profileColumn{
		Name:          c.name,
		Type:          c.typ,
		NullCount:     c.nullCount,
		BlankCount:    c.blankCount,
		DistinctCount: int64(len(c.distinct)),
		NumericCount:  c.numeric,
		Warnings:      []string{},
	}
	if c.numeric > 0 && c.nonNumeric > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("mixed numeric and non-numeric values (%d numeric, %d non-numeric)", c.numeric, c.nonNumeric))
	}
	if c.nullLike > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("%d value(s) look like null placeholders (e.g. NULL, N/A)", c.nullLike))
	}
	if c.spacecnt > 0 {
		pc.Warnings = append(pc.Warnings, fmt.Sprintf("%d value(s) have leading or trailing whitespace", c.spacecnt))
	}
	return pc
}

// isNumericValue reports whether s is a finite decimal number. It rejects the
// Go-specific float spellings ParseFloat also accepts but data rarely means as
// numbers: hexadecimal floats ("0x1p4"), underscore digit separators
// ("1_000"), and the Infinity/NaN words. This keeps the profile's numeric count
// aligned with what a human would call a number.
func isNumericValue(s string) bool {
	if strings.ContainsAny(s, "xXpP_") {
		return false
	}
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
