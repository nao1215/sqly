package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// outputFormatText is the human-readable value accepted by --compare-format and
// --profile-format (the default is JSON).
const outputFormatText = "text"

// compareColumnTypeChange records a column present in both tables whose declared
// type differs between them.
type compareColumnTypeChange struct {
	Name      string `json:"name"`
	LeftType  string `json:"left_type"`
	RightType string `json:"right_type"`
}

// compareSchema is the schema-difference section of a compare report.
type compareSchema struct {
	Equal            bool                      `json:"equal"`
	LeftOnlyColumns  []string                  `json:"left_only_columns"`
	RightOnlyColumns []string                  `json:"right_only_columns"`
	TypeChanges      []compareColumnTypeChange `json:"type_changes"`
}

// compareRowCount is the row-count section of a compare report. Delta is
// right minus left.
type compareRowCount struct {
	Left  int64 `json:"left"`
	Right int64 `json:"right"`
	Delta int64 `json:"delta"`
}

// compareModifiedRow describes a key present in both tables whose non-key values
// differ. Left and Right hold the full row from each side.
type compareModifiedRow struct {
	Key   string            `json:"key"`
	Left  map[string]string `json:"left"`
	Right map[string]string `json:"right"`
}

// compareRows is the keyed-row section of a compare report, present only when a
// key column is given.
type compareRows struct {
	Key      string               `json:"key"`
	Added    []map[string]string  `json:"added"`
	Removed  []map[string]string  `json:"removed"`
	Modified []compareModifiedRow `json:"modified"`
}

// compareReport is the top-level JSON contract produced by --compare.
type compareReport struct {
	Left     string          `json:"left"`
	Right    string          `json:"right"`
	Schema   compareSchema   `json:"schema"`
	RowCount compareRowCount `json:"row_count"`
	Rows     *compareRows    `json:"rows,omitempty"`
}

// validateCompareFlags rejects --compare combined with flags that ask for a
// different action or side effect, mirroring --inspect. --compare imports the
// inputs, prints its own report, and exits, so a query, output, or write-back
// flag would otherwise be silently discarded.
func (s *Shell) validateCompareFlags() error {
	if !s.argument.CompareFlag {
		return nil
	}
	switch {
	case s.argument.InspectFlag:
		return errors.New("--compare cannot be combined with --inspect")
	case s.argument.Query != "":
		return errors.New("--compare cannot be combined with --sql")
	case s.argument.SQLFilePath != "":
		return errors.New("--compare cannot be combined with --sql-file")
	case s.argument.Output != nil && s.argument.Output.FilePath != "":
		return errors.New("--compare cannot be combined with --output")
	case s.argument.SaveInPlace:
		return errors.New("--compare cannot be combined with --save")
	case s.argument.SaveDir != "":
		return errors.New("--compare cannot be combined with --save-dir")
	case s.argument.Output != nil && s.argument.Output.Mode != model.PrintModeTable:
		return fmt.Errorf("--compare cannot be combined with an output mode flag (--%s)", s.argument.Output.Mode.String())
	}
	return nil
}

// runCompare compares two imported tables and prints a report (JSON by default,
// or a human-readable summary with --compare-format text). It is the
// non-interactive comparison path for scripts: schema and row-count differences
// are always reported, and keyed row differences are added when --compare-key is
// given.
func (s *Shell) runCompare(ctx context.Context) error {
	left, right, err := s.resolveCompareTables(ctx)
	if err != nil {
		return err
	}

	report, err := s.buildCompareReport(ctx, left, right, s.argument.CompareKey)
	if err != nil {
		return err
	}

	if s.argument.CompareFormat == outputFormatText {
		fmt.Fprint(config.Stdout, renderCompareText(report))
		return nil
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode compare report: %w", err)
	}
	fmt.Fprintln(config.Stdout, string(encoded))
	return nil
}

// resolveCompareTables determines the two tables to compare. With
// --compare-tables it uses the named pair; otherwise it requires exactly two
// imported tables. Each name is validated to exist so a typo or an ambiguous
// multi-table import (for example an ACH file that produced several tables) fails
// with a clear error instead of comparing the wrong pair.
func (s *Shell) resolveCompareTables(ctx context.Context) (string, string, error) {
	if spec := strings.TrimSpace(s.argument.CompareTables); spec != "" {
		parts := strings.Split(spec, ",")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("--compare-tables must name exactly two tables as \"left,right\", got %q", spec)
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			return "", "", fmt.Errorf("--compare-tables must name exactly two tables as \"left,right\", got %q", spec)
		}
		if !s.objectExists(ctx, left) {
			return "", "", fmt.Errorf("compare table %q not found", left)
		}
		if !s.objectExists(ctx, right) {
			return "", "", fmt.Errorf("compare table %q not found", right)
		}
		return left, right, nil
	}

	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to list tables: %w", err)
	}
	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	switch len(names) {
	case 0:
		return "", "", errors.New("no tables to compare: provide two inputs or use --compare-tables")
	case 2:
		// Sort for a deterministic left/right; --compare-tables overrides the order.
		sort.Strings(names)
		return names[0], names[1], nil
	default:
		sort.Strings(names)
		return "", "", fmt.Errorf("--compare needs exactly two tables but found %d (%s); name the pair with --compare-tables \"left,right\"",
			len(names), strings.Join(names, ", "))
	}
}

// buildCompareReport assembles the comparison of two existing tables.
func (s *Shell) buildCompareReport(ctx context.Context, left, right, key string) (compareReport, error) {
	leftCols, err := s.inspectColumns(ctx, left)
	if err != nil {
		return compareReport{}, err
	}
	rightCols, err := s.inspectColumns(ctx, right)
	if err != nil {
		return compareReport{}, err
	}

	leftCount, err := s.inspectRowCount(ctx, left)
	if err != nil {
		return compareReport{}, err
	}
	rightCount, err := s.inspectRowCount(ctx, right)
	if err != nil {
		return compareReport{}, err
	}

	report := compareReport{
		Left:     left,
		Right:    right,
		Schema:   compareSchemas(leftCols, rightCols),
		RowCount: compareRowCount{Left: leftCount, Right: rightCount, Delta: rightCount - leftCount},
	}

	if key != "" {
		leftTable, err := s.selectAll(ctx, left)
		if err != nil {
			return compareReport{}, err
		}
		rightTable, err := s.selectAll(ctx, right)
		if err != nil {
			return compareReport{}, err
		}
		rows, err := compareKeyedRows(left, right, leftTable, rightTable, key)
		if err != nil {
			return compareReport{}, err
		}
		report.Rows = rows
	}
	return report, nil
}

// selectAll returns every row of a table for row-level comparison.
func (s *Shell) selectAll(ctx context.Context, name string) (*model.Table, error) {
	quoted := s.usecases.importer.QuoteIdentifier(name)
	table, err := s.usecases.query.Query(ctx, "SELECT * FROM "+quoted)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows of %s: %w", name, err)
	}
	return table, nil
}

// compareSchemas reports the column-level differences between two schemas: which
// columns are unique to each side and which shared columns changed type. Column
// order follows each table's definition order; the unique/changed lists are
// emitted in left-then-right order so the report is deterministic.
func compareSchemas(left, right []inspectColumn) compareSchema {
	rightByName := make(map[string]inspectColumn, len(right))
	for _, c := range right {
		rightByName[c.Name] = c
	}
	leftByName := make(map[string]inspectColumn, len(left))
	for _, c := range left {
		leftByName[c.Name] = c
	}

	var leftOnly []string
	var typeChanges []compareColumnTypeChange
	for _, c := range left {
		rc, ok := rightByName[c.Name]
		if !ok {
			leftOnly = append(leftOnly, c.Name)
			continue
		}
		if rc.Type != c.Type {
			typeChanges = append(typeChanges, compareColumnTypeChange{Name: c.Name, LeftType: c.Type, RightType: rc.Type})
		}
	}
	var rightOnly []string
	for _, c := range right {
		if _, ok := leftByName[c.Name]; !ok {
			rightOnly = append(rightOnly, c.Name)
		}
	}

	return compareSchema{
		Equal:            len(leftOnly) == 0 && len(rightOnly) == 0 && len(typeChanges) == 0,
		LeftOnlyColumns:  leftOnly,
		RightOnlyColumns: rightOnly,
		TypeChanges:      typeChanges,
	}
}

// compareKeyedRows diffs two tables by a key column. It returns the rows added
// (key only in right), removed (key only in left), and modified (same key, any
// differing value). The key column must exist in both tables and be unique on
// each side, otherwise the comparison is ambiguous and an error is returned.
func compareKeyedRows(leftName, rightName string, left, right *model.Table, key string) (*compareRows, error) {
	leftIdx := indexOfColumn(left.Header(), key)
	if leftIdx < 0 {
		return nil, fmt.Errorf("compare key %q not found in table %s", key, leftName)
	}
	rightIdx := indexOfColumn(right.Header(), key)
	if rightIdx < 0 {
		return nil, fmt.Errorf("compare key %q not found in table %s", key, rightName)
	}

	leftByKey, err := indexRowsByKey(left, leftIdx, leftName, key)
	if err != nil {
		return nil, err
	}
	rightByKey, err := indexRowsByKey(right, rightIdx, rightName, key)
	if err != nil {
		return nil, err
	}

	rows := &compareRows{
		Key:      key,
		Added:    []map[string]string{},
		Removed:  []map[string]string{},
		Modified: []compareModifiedRow{},
	}

	leftKeys := sortedKeys(leftByKey)
	for _, k := range leftKeys {
		lrow := leftByKey[k]
		rrow, ok := rightByKey[k]
		if !ok {
			rows.Removed = append(rows.Removed, rowMap(left.Header(), lrow))
			continue
		}
		lm := rowMap(left.Header(), lrow)
		rm := rowMap(right.Header(), rrow)
		if !rowMapsEqual(lm, rm) {
			rows.Modified = append(rows.Modified, compareModifiedRow{Key: k, Left: lm, Right: rm})
		}
	}
	for _, k := range sortedKeys(rightByKey) {
		if _, ok := leftByKey[k]; !ok {
			rows.Added = append(rows.Added, rowMap(right.Header(), rightByKey[k]))
		}
	}
	return rows, nil
}

// indexRowsByKey maps each row to its key-column value, rejecting a duplicate key
// (which would make the keyed comparison ambiguous).
func indexRowsByKey(t *model.Table, keyIdx int, name, key string) (map[string]model.Record, error) {
	out := make(map[string]model.Record, len(t.Records()))
	for _, rec := range t.Records() {
		if keyIdx >= len(rec) {
			continue
		}
		k := rec[keyIdx]
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("compare key %q is not unique in table %s (value %q appears more than once)", key, name, k)
		}
		out[k] = rec
	}
	return out, nil
}

// indexOfColumn returns the position of name in header, or -1.
func indexOfColumn(header model.Header, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

// rowMap pairs a record's values with their column names.
func rowMap(header model.Header, rec model.Record) map[string]string {
	m := make(map[string]string, len(header))
	for i, h := range header {
		if i < len(rec) {
			m[h] = rec[i]
		}
	}
	return m
}

// rowMapsEqual reports whether two row maps have the same keys and values.
func rowMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// sortedKeys returns the keys of a row index in deterministic order.
func sortedKeys(m map[string]model.Record) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renderCompareText renders a human-readable summary of a compare report.
func renderCompareText(r compareReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "compare %s -> %s\n", r.Left, r.Right)
	if r.Schema.Equal {
		fmt.Fprintln(&b, "schema: identical")
	} else {
		fmt.Fprintln(&b, "schema: changed")
		if len(r.Schema.LeftOnlyColumns) > 0 {
			fmt.Fprintf(&b, "  columns only in %s: %s\n", r.Left, strings.Join(r.Schema.LeftOnlyColumns, ", "))
		}
		if len(r.Schema.RightOnlyColumns) > 0 {
			fmt.Fprintf(&b, "  columns only in %s: %s\n", r.Right, strings.Join(r.Schema.RightOnlyColumns, ", "))
		}
		for _, tc := range r.Schema.TypeChanges {
			fmt.Fprintf(&b, "  type change %s: %s -> %s\n", tc.Name, tc.LeftType, tc.RightType)
		}
	}
	fmt.Fprintf(&b, "rows: %d -> %d (delta %+d)\n", r.RowCount.Left, r.RowCount.Right, r.RowCount.Delta)
	if r.Rows != nil {
		fmt.Fprintf(&b, "keyed by %s: %d added, %d removed, %d modified\n",
			r.Rows.Key, len(r.Rows.Added), len(r.Rows.Removed), len(r.Rows.Modified))
	}
	return b.String()
}
