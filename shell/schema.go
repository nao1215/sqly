package shell

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// schemaCommand prints the CREATE TABLE statement of a table.
//
// In JSON/NDJSON mode it emits a structured `{table, schema}` object so the
// schema is machine-readable; in other modes it prints the raw CREATE statement,
// which is more readable than wrapping a multi-column DDL string in a cell.
func (c CommandList) schemaCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .schema TABLE_NAME")
		return nil
	}
	if len(argv) > 1 {
		return fmt.Errorf(".schema accepts a single table name, got %d arguments", len(argv))
	}
	tableName := argv[0]

	createSQL, err := s.tableCreateStatement(ctx, tableName)
	if err != nil {
		return err
	}

	if isStructuredMode(s.state.mode.PrintMode) {
		t := model.NewTable("schema", model.Header{"table", "schema"}, []model.Record{{tableName, createSQL}})
		return t.Print(config.Stdout, s.state.mode.PrintMode)
	}
	fmt.Fprintln(config.Stdout, createSQL)
	return nil
}

// isStructuredMode reports whether the output mode is a machine-readable JSON form.
func isStructuredMode(m model.PrintMode) bool {
	return m == model.PrintModeJSON || m == model.PrintModeNDJSON
}

// tableCreateStatement returns the table's CREATE statement. It prefers the
// statement SQLite stored in sqlite_master; if that is unavailable it builds a
// functionally equivalent statement from column metadata. Returns an error when
// the table does not exist.
func (s *Shell) tableCreateStatement(ctx context.Context, tableName string) (string, error) {
	// String literal: escape single quotes to query sqlite_master safely.
	literal := "'" + strings.ReplaceAll(tableName, "'", "''") + "'"
	master, err := s.usecases.query.Query(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = "+literal)
	if err != nil {
		return "", err
	}
	if recs := master.Records(); len(recs) > 0 && len(recs[0]) > 0 && recs[0][0] != "" {
		return recs[0][0], nil
	}

	// Fallback: synthesize from column metadata (also detects a missing table).
	cols, err := s.tableColumns(ctx, tableName)
	if err != nil {
		return "", err
	}
	if len(cols.Records()) == 0 {
		return "", fmt.Errorf("no such table: %s", tableName)
	}
	return s.buildCreateStatement(tableName, cols), nil
}

// buildCreateStatement assembles a CREATE TABLE statement from PRAGMA
// table_info records (columns: cid, name, type, notnull, dflt_value, pk) so the
// fallback stays faithful to the real schema: it preserves quoted identifiers,
// detected types, NOT NULL, DEFAULT, and the primary key. A single-column key is
// written inline; a composite key becomes a table-level PRIMARY KEY clause.
func (s *Shell) buildCreateStatement(tableName string, cols *model.Table) string {
	pkCols := primaryKeyColumns(cols)

	var b strings.Builder
	b.WriteString("CREATE TABLE ")
	b.WriteString(s.usecases.importer.QuoteIdentifier(tableName))
	b.WriteString(" (")
	for i, rec := range cols.Records() {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(s.usecases.importer.QuoteIdentifier(rec[1]))
		if colType := rec[2]; colType != "" {
			b.WriteString(" ")
			b.WriteString(colType)
		}
		if rec[3] == "1" {
			b.WriteString(" NOT NULL")
		}
		if rec[4] != "" {
			b.WriteString(" DEFAULT ")
			b.WriteString(rec[4]) // PRAGMA already gives a SQL-literal token
		}
		if len(pkCols) == 1 && rec[5] != "0" && rec[5] != "" {
			b.WriteString(" PRIMARY KEY")
		}
	}
	if len(pkCols) > 1 {
		b.WriteString(", PRIMARY KEY (")
		for i, name := range pkCols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(s.usecases.importer.QuoteIdentifier(name))
		}
		b.WriteString(")")
	}
	b.WriteString(")")
	return b.String()
}

// primaryKeyColumns returns the primary-key column names in key order (PRAGMA's
// pk value is the 1-based position within the key, 0 when not part of it).
func primaryKeyColumns(cols *model.Table) []string {
	type pkCol struct {
		name string
		pos  int
	}
	var pks []pkCol
	for _, rec := range cols.Records() {
		if rec[5] == "0" || rec[5] == "" {
			continue
		}
		pos, err := strconv.Atoi(rec[5])
		if err != nil {
			continue
		}
		pks = append(pks, pkCol{name: rec[1], pos: pos})
	}
	sort.Slice(pks, func(i, j int) bool { return pks[i].pos < pks[j].pos })
	names := make([]string, len(pks))
	for i, p := range pks {
		names[i] = p.name
	}
	return names
}
