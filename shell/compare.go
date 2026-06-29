package shell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// outputFormatText is the human-readable value accepted by --compare-format and
// --profile-format (the default is JSON).
const outputFormatText = "text"

// outputModeFlagName returns the flag name that selected an output mode, naming
// the typed JSON variants (--json-typed / --ndjson-typed) rather than the plain
// base mode, so a conflict error names the flag the user actually passed.
func outputModeFlagName(o *config.Output) string {
	if o == nil {
		return ""
	}
	if o.JSONTyped {
		switch o.Mode {
		case model.PrintModeJSON:
			return outputModeJSONTyped
		case model.PrintModeNDJSON:
			return outputModeNDJSONTyped
		}
	}
	return o.Mode.String()
}

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

// compareRow is one row keyed by column name. A nil value is a SQL NULL (emitted
// as JSON null), distinct from a pointer to "" (an empty string), so a change
// between NULL and "" is detected and reported.
type compareRow map[string]*string

// compareModifiedRow describes a key present in both tables whose non-key values
// differ. Left and Right hold the full row from each side.
type compareModifiedRow struct {
	Key   string     `json:"key"`
	Left  compareRow `json:"left"`
	Right compareRow `json:"right"`
}

// compareRows is the keyed-row section of a compare report, present only when a
// key column is given.
type compareRows struct {
	Key      string               `json:"key"`
	Added    []compareRow         `json:"added"`
	Removed  []compareRow         `json:"removed"`
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
		return fmt.Errorf("--compare cannot be combined with an output mode flag (--%s)", outputModeFlagName(s.argument.Output))
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
		leftName, ok := s.resolveTableNameCI(ctx, left)
		if !ok {
			return "", "", fmt.Errorf("compare table %q not found", left)
		}
		rightName, ok := s.resolveTableNameCI(ctx, right)
		if !ok {
			return "", "", fmt.Errorf("compare table %q not found", right)
		}
		return leftName, rightName, nil
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
		// TablesName returns tables in import order (CLI input order), so left/right
		// follow the order the user typed. --compare-tables overrides the pair and order.
		return names[0], names[1], nil
	default:
		sort.Strings(names)
		return "", "", fmt.Errorf("--compare needs exactly two tables but found %d (%s); name the pair with --compare-tables \"left,right\"",
			len(names), strings.Join(names, ", "))
	}
}

// resolveTableNameCI resolves a user-supplied table name to the actual stored
// name case-insensitively, so --compare-tables follows SQLite identifier
// semantics where "USER" names the table imported as "user". It returns the
// canonical name and true on a match, prefers the temp schema over main, and
// returns false when no table or view matches. Returning the stored case keeps
// the comparison report's left/right labels consistent with .tables output.
func (s *Shell) resolveTableNameCI(ctx context.Context, name string) (string, bool) {
	literal := "'" + strings.ReplaceAll(name, "'", "''") + "'"
	res, err := s.usecases.query.Query(ctx,
		"SELECT name FROM sqlite_temp_master WHERE name = "+literal+" COLLATE NOCASE AND type IN ('table', 'view') "+
			"UNION ALL SELECT name FROM sqlite_master WHERE name = "+literal+" COLLATE NOCASE AND type IN ('table', 'view')")
	if err != nil {
		return "", false
	}
	records := res.Records()
	if len(records) == 0 || len(records[0]) == 0 {
		return "", false
	}
	return records[0][0], true
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
		rows, err := s.diffKeyedRowsSQL(ctx, left, right, key, leftCols, rightCols)
		if err != nil {
			return compareReport{}, err
		}
		report.Rows = rows
	}
	return report, nil
}

// diffKeyedRowsSQL builds the keyed-row diff by streaming both tables instead of
// loading them into full keyed maps. The added, removed, and modified rows are
// the only rows copied into Go, so peak memory scales with the size of the diff
// and the key set rather than with both full tables.
//
// The first pass over the left side records, per key, only a row fingerprint, not
// the row data. The pass over the right side compares each row against that
// fingerprint on the fly: a key not on the left is added and a fingerprint
// mismatch is a modification candidate, and only those right rows are retained. A
// final left pass fetches the few rows behind the removed and candidate keys.
// Candidates are then re-checked with the exact rowMapsEqual, so a fingerprint
// collision can never report a false change. The output is identical to the
// in-memory diffKeyedRows path.
func (s *Shell) diffKeyedRowsSQL(ctx context.Context, left, right, key string, leftCols, rightCols []inspectColumn) (*compareRows, error) {
	leftKeyCol, err := resolveKeyColumn(leftCols, key, left)
	if err != nil {
		return nil, err
	}
	rightKeyCol, err := resolveKeyColumn(rightCols, key, right)
	if err != nil {
		return nil, err
	}

	// When the column name sets differ, the in-memory path treats every shared row
	// as modified (rowMapsEqual returns false on mismatched maps) regardless of
	// values, so a shared key is always a candidate. Otherwise only a fingerprint
	// mismatch makes a shared key a candidate.
	columnsMatch := sameColumnNames(leftCols, rightCols)

	leftFP := make(map[string]uint64)
	err = s.streamKeyedRows(ctx, left, leftKeyCol, leftCols, func(k string, record []string, nulls []bool) error {
		if _, dup := leftFP[k]; dup {
			return duplicateKeyError(leftKeyCol, left, k)
		}
		leftFP[k] = rowFingerprint(leftCols, record, nulls)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Pass over the right side: classify added and candidate rows and retain only
	// those, and remember which left keys were seen so removed keys are the left
	// keys the right side never produced.
	addedRows := make(map[string]compareRow)
	candidateRows := make(map[string]compareRow)
	seen := make(map[string]struct{}, len(leftFP))
	err = s.streamKeyedRows(ctx, right, rightKeyCol, rightCols, func(k string, record []string, nulls []bool) error {
		if _, dup := seen[k]; dup {
			return duplicateKeyError(rightKeyCol, right, k)
		}
		seen[k] = struct{}{}
		lfp, shared := leftFP[k]
		switch {
		case !shared:
			addedRows[k] = recordToCompareRow(rightCols, record, nulls)
		case !columnsMatch || lfp != rowFingerprint(rightCols, record, nulls):
			candidateRows[k] = recordToCompareRow(rightCols, record, nulls)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Fetch only the left rows the report needs: removed keys and candidate keys.
	leftWanted := make(map[string]struct{}, len(leftFP))
	for k := range leftFP {
		if _, ok := seen[k]; !ok {
			leftWanted[k] = struct{}{} // removed
			continue
		}
		if _, ok := candidateRows[k]; ok {
			leftWanted[k] = struct{}{} // candidate
		}
	}
	leftRows, err := s.rowsForKeys(ctx, left, leftKeyCol, leftCols, leftWanted)
	if err != nil {
		return nil, err
	}

	rows := &compareRows{
		Key:      key,
		Added:    []compareRow{},
		Removed:  []compareRow{},
		Modified: []compareModifiedRow{},
	}
	for _, k := range sortedKeys(leftFP) {
		if _, ok := seen[k]; !ok {
			rows.Removed = append(rows.Removed, leftRows[k])
			continue
		}
		rrow, ok := candidateRows[k]
		if !ok {
			continue
		}
		lrow := leftRows[k]
		// Re-check exactly so a fingerprint collision is not reported as modified.
		if rowMapsEqual(lrow, rrow) {
			continue
		}
		rows.Modified = append(rows.Modified, compareModifiedRow{Key: k, Left: lrow, Right: rrow})
	}
	for _, k := range sortedKeys(addedRows) {
		rows.Added = append(rows.Added, addedRows[k])
	}
	return rows, nil
}

// duplicateKeyError reports a duplicate key value with the same message the
// in-memory path uses, so the comparison fails the same way.
func duplicateKeyError(keyCol, name, value string) error {
	return fmt.Errorf("compare key %q is not unique in table %s (value %q appears more than once)", keyCol, name, value)
}

// keyExpr renders the SQL text of a row's key: the key cell cast to TEXT with a
// SQL NULL mapped to the empty string. This matches the in-memory key, where a
// NULL key and an empty-string key share the same map key.
func keyExpr(quotedCol string) string {
	return "COALESCE(CAST(" + quotedCol + " AS TEXT), '')"
}

// streamKeyedRows streams a table once, invoking fn with each row's key string
// (the COALESCE-to-empty text of the key cell), its cells, and its SQL NULL flags.
// The key is appended as a trailing column so the callback can classify a row
// without a second lookup. The leading columns line up with cols, which comes from
// PRAGMA table_info in the same order as SELECT *.
func (s *Shell) streamKeyedRows(ctx context.Context, name, keyCol string, cols []inspectColumn, fn func(key string, record []string, nulls []bool) error) error {
	quote := s.usecases.importer.QuoteIdentifier
	quotedTable := quote(name)
	quotedKey := quote(keyCol)
	keyIdx := len(cols) // the appended key expression is the last column
	return s.usecases.query.QueryStream(ctx,
		"SELECT *, "+keyExpr(quotedKey)+" FROM "+quotedTable,
		func(record []string, nulls []bool) error {
			if keyIdx >= len(record) {
				return nil
			}
			return fn(record[keyIdx], record, nulls)
		})
}

// recordToCompareRow pairs a streamed row's cells with their column names,
// preserving the SQL NULL/empty-string distinction the same way rowMap does: a
// NULL cell maps to nil, a value maps to a pointer to its string.
func recordToCompareRow(cols []inspectColumn, record []string, nulls []bool) compareRow {
	row := make(compareRow, len(cols))
	for i, c := range cols {
		if i >= len(record) {
			break
		}
		if i < len(nulls) && nulls[i] {
			row[c.Name] = nil
			continue
		}
		v := record[i]
		row[c.Name] = &v
	}
	return row
}

// rowFingerprint hashes a row's cells (by column-definition order) and per-cell
// SQL NULL flags with FNV-1a. It is only a fast inequality filter: matching
// fingerprints are re-checked exactly with rowMapsEqual, so a collision cannot
// produce a wrong report.
func rowFingerprint(cols []inspectColumn, record []string, nulls []bool) uint64 {
	h := fnv.New64a()
	for i := range cols {
		if i < len(nulls) && nulls[i] {
			_, _ = h.Write([]byte{0})
			continue
		}
		_, _ = h.Write([]byte{1})
		if i < len(record) {
			_, _ = h.Write([]byte(record[i]))
		}
		_, _ = h.Write([]byte{0}) // separator so concatenations cannot collide
	}
	return h.Sum64()
}

// rowsForKeys streams a table once and returns the rows whose key is in wanted,
// each as a compareRow. Rows not in the diff are skipped, so only the wanted rows
// are copied into Go.
func (s *Shell) rowsForKeys(ctx context.Context, name, keyCol string, cols []inspectColumn, wanted map[string]struct{}) (map[string]compareRow, error) {
	if len(wanted) == 0 {
		return map[string]compareRow{}, nil
	}
	out := make(map[string]compareRow, len(wanted))
	err := s.streamKeyedRows(ctx, name, keyCol, cols, func(k string, record []string, nulls []bool) error {
		if _, ok := wanted[k]; !ok {
			return nil
		}
		out[k] = recordToCompareRow(cols, record, nulls)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveKeyColumn finds the key column in a table's columns case-insensitively,
// following SQLite identifier semantics where "ID" resolves the column "id". It
// returns the canonical stored name so later quoting matches the real column.
func resolveKeyColumn(cols []inspectColumn, key, name string) (string, error) {
	for _, c := range cols {
		if strings.EqualFold(c.Name, key) {
			return c.Name, nil
		}
	}
	return "", fmt.Errorf("compare key %q not found in table %s", key, name)
}

// sameColumnNames reports whether two column lists have the same set of names.
// The in-memory diff treats rows with different column sets as always modified,
// so the SQL path can only compare values column-by-column when the sets match.
func sameColumnNames(left, right []inspectColumn) bool {
	if len(left) != len(right) {
		return false
	}
	set := make(map[string]struct{}, len(left))
	for _, c := range left {
		set[c.Name] = struct{}{}
	}
	for _, c := range right {
		if _, ok := set[c.Name]; !ok {
			return false
		}
	}
	return true
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
	leftByKey, err := keyedRowsFromTable(left, key, leftName)
	if err != nil {
		return nil, err
	}
	rightByKey, err := keyedRowsFromTable(right, key, rightName)
	if err != nil {
		return nil, err
	}
	return diffKeyedRows(key, leftByKey, rightByKey), nil
}

// keyedRowsFromTable indexes a table's rows by the key column's value, rejecting
// a duplicate key (which would make the keyed comparison ambiguous). A SQL NULL
// key is keyed under the empty string, matching how a NULL key renders. Each row
// is materialized into a compareRow so the source table can be released.
func keyedRowsFromTable(t *model.Table, key, name string) (map[string]compareRow, error) {
	keyIdx := indexOfColumn(t.Header(), key)
	if keyIdx < 0 {
		return nil, fmt.Errorf("compare key %q not found in table %s", key, name)
	}
	records := t.Records()
	out := make(map[string]compareRow, len(records))
	for i, rec := range records {
		if keyIdx >= len(rec) {
			continue
		}
		k := rec[keyIdx]
		if _, dup := out[k]; dup {
			return nil, fmt.Errorf("compare key %q is not unique in table %s (value %q appears more than once)", key, name, k)
		}
		out[k] = rowMap(t, i)
	}
	return out, nil
}

// diffKeyedRows builds the keyed row diff from two already-indexed sides. Removed
// and modified rows follow the left side's sorted keys, added rows the right
// side's, so the report is deterministic.
func diffKeyedRows(key string, leftByKey, rightByKey map[string]compareRow) *compareRows {
	rows := &compareRows{
		Key:      key,
		Added:    []compareRow{},
		Removed:  []compareRow{},
		Modified: []compareModifiedRow{},
	}

	for _, k := range sortedKeys(leftByKey) {
		lrow := leftByKey[k]
		rrow, ok := rightByKey[k]
		if !ok {
			rows.Removed = append(rows.Removed, lrow)
			continue
		}
		if !rowMapsEqual(lrow, rrow) {
			rows.Modified = append(rows.Modified, compareModifiedRow{Key: k, Left: lrow, Right: rrow})
		}
	}
	for _, k := range sortedKeys(rightByKey) {
		if _, ok := leftByKey[k]; !ok {
			rows.Added = append(rows.Added, rightByKey[k])
		}
	}
	return rows
}

// indexOfColumn returns the position of name in header, or -1. The match is
// case-insensitive so --compare-key follows SQLite identifier semantics, where
// "ID" resolves the same column as "id". SQLite forbids two columns that differ
// only by case, so the case-insensitive match stays unambiguous.
func indexOfColumn(header model.Header, name string) int {
	for i, h := range header {
		if strings.EqualFold(h, name) {
			return i
		}
	}
	return -1
}

// rowMap pairs the values of the row at rowIdx with their column names,
// preserving the SQL NULL/empty-string distinction: a NULL cell maps to nil, a
// real value maps to a pointer to its string.
func rowMap(t *model.Table, rowIdx int) compareRow {
	header := t.Header()
	rec := t.Records()[rowIdx]
	m := make(compareRow, len(header))
	for i, h := range header {
		if t.IsNull(rowIdx, i) {
			m[h] = nil
			continue
		}
		if i < len(rec) {
			v := rec[i]
			m[h] = &v
		}
	}
	return m
}

// rowMapsEqual reports whether two row maps have the same keys and values,
// treating two NULLs (nil) as equal and a NULL as different from any string.
func rowMapsEqual(a, b compareRow) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		switch {
		case av == nil && bv == nil:
			// both NULL: equal
		case av == nil || bv == nil:
			return false // one NULL, one not
		case *av != *bv:
			return false
		}
	}
	return true
}

// sortedKeys returns the keys of a string-keyed map in deterministic order.
func sortedKeys[V any](m map[string]V) []string {
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
