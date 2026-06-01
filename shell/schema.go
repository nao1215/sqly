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

// tableCreateStatement returns the object's stored CREATE statement. It prefers
// the exact SQL SQLite stored, so a VIEW prints "CREATE VIEW ..." (not a
// synthesized table) and a TEMP or constrained table prints its real definition
// instead of a lossy reconstruction. Only when no stored SQL is found does it fall
// back to synthesizing from column metadata. A schema-qualified name (main.user,
// temp.t) is resolved against that schema. Returns an error when the object does
// not exist. Ref #445, #451, #463, #464.
func (s *Shell) tableCreateStatement(ctx context.Context, tableName string) (string, error) {
	schema, object := splitTableQualifier(tableName)
	stored, err := s.storedCreateSQL(ctx, schema, object)
	if err != nil {
		return "", err
	}
	if stored != "" {
		return stored, nil
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

// storedCreateSQL returns the CREATE statement SQLite stored for a table or view,
// or "" when none is found. It reads the schema's sqlite_master (or, for a TEMP
// object or an unqualified name, sqlite_temp_master) so the real definition wins
// over the synthesized fallback: a view yields its CREATE VIEW, and a constrained
// or TEMP table yields its original DDL including UNIQUE/CHECK constraints the
// fallback cannot reconstruct. Ref #451, #463, #464.
func (s *Shell) storedCreateSQL(ctx context.Context, schema, object string) (string, error) {
	masters := []string{"sqlite_master", "sqlite_temp_master"}
	if schema != "" {
		masters = []string{s.usecases.importer.QuoteIdentifier(schema) + ".sqlite_master"}
	}
	// String literal: escape single quotes to query the master tables safely.
	literal := "'" + strings.ReplaceAll(object, "'", "''") + "'"
	for _, master := range masters {
		res, err := s.usecases.query.Query(ctx,
			"SELECT sql FROM "+master+" WHERE name = "+literal+" AND type IN ('table', 'view') AND sql IS NOT NULL")
		if err != nil {
			return "", err
		}
		if recs := res.Records(); len(recs) > 0 && len(recs[0]) > 0 && recs[0][0] != "" {
			return recs[0][0], nil
		}
	}
	return "", nil
}

// splitTableQualifier splits a possibly schema-qualified table reference such as
// "main.user" or "temp.t" into its schema and object name. A bare name yields an
// empty schema. The split is on the first dot, which sqly never produces inside an
// imported table name (dots are sanitized to "_"), so a dot signals a schema
// qualifier. SQLite accepts a schema-qualified name in SQL, so the helper commands
// accept it too. Ref #445, #446, #447, #448.
func splitTableQualifier(name string) (schema, object string) {
	if i := strings.IndexByte(name, '.'); i > 0 && i < len(name)-1 {
		return name[:i], name[i+1:]
	}
	return "", name
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
