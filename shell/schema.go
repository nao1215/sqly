package shell

import (
	"context"
	"errors"
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
		// A missing required argument is a command error, not a no-op: returning
		// nil here would let a batch script continue and exit 0 after silently
		// skipping the command. The usage text rides on the error path instead.
		return errors.New(".schema requires a table name\n[Usage]\n  .schema TABLE_NAME")
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
// not exist.
func (s *Shell) tableCreateStatement(ctx context.Context, tableName string) (string, error) {
	schema, object := s.resolveObjectName(ctx, tableName)
	stored, isTemp, err := s.storedCreateSQL(ctx, schema, object)
	if err != nil {
		return "", err
	}
	if stored != "" {
		// SQLite stores a temp object's SQL with the TEMP keyword removed, so
		// re-insert it for a temp object to keep the reported schema faithful and
		// round-trippable to the same kind of object.
		if isTemp {
			stored = injectTempKeyword(stored)
		}
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
// whether the matched object is a TEMP object, or "" when none is found. It reads
// the right master table so the real definition wins over the synthesized
// fallback: a view yields its CREATE VIEW, and a constrained or TEMP table yields
// its original DDL including UNIQUE/CHECK constraints the fallback cannot
// reconstruct.
//
// An unqualified name is resolved against sqlite_temp_master before sqlite_master,
// matching SQLite's own temp-before-main lookup, so a TEMP object shadowing an
// imported main table reports the temp definition the session will actually query.
// A "temp."/"main."-qualified name is resolved against just that schema.
func (s *Shell) storedCreateSQL(ctx context.Context, schema, object string) (sqlText string, isTemp bool, err error) {
	type masterSource struct {
		table string
		temp  bool
	}
	var sources []masterSource
	switch {
	case schema == "":
		sources = []masterSource{{"sqlite_temp_master", true}, {"sqlite_master", false}}
	case strings.EqualFold(schema, "temp"):
		sources = []masterSource{{"sqlite_temp_master", true}}
	default: // main
		sources = []masterSource{{"sqlite_master", false}}
	}
	// String literal: escape single quotes to query the master tables safely.
	literal := "'" + strings.ReplaceAll(object, "'", "''") + "'"
	for _, src := range sources {
		// Match the name case-insensitively (COLLATE NOCASE) so the stored DDL
		// still wins when the user spells the object in a different case, for
		// example ".schema V" for a view created as "v". SQLite forbids two
		// objects whose names differ only by case, so the match is unambiguous.
		res, err := s.usecases.query.Query(ctx,
			"SELECT sql FROM "+src.table+" WHERE name = "+literal+" COLLATE NOCASE AND type IN ('table', 'view') AND sql IS NOT NULL")
		if err != nil {
			return "", false, err
		}
		if recs := res.Records(); len(recs) > 0 && len(recs[0]) > 0 && recs[0][0] != "" {
			return recs[0][0], src.temp, nil
		}
	}
	return "", false, nil
}

// injectTempKeyword re-inserts the TEMP keyword after CREATE, because SQLite
// strips TEMP/TEMPORARY from the SQL it stores for a temp object. The guard
// clauses make it a no-op for a non-CREATE or already temp-qualified statement.
func injectTempKeyword(createSQL string) string {
	i := 0
	for i < len(createSQL) && (createSQL[i] == ' ' || createSQL[i] == '\t' || createSQL[i] == '\n' || createSQL[i] == '\r') {
		i++
	}
	const kw = "CREATE"
	if !strings.HasPrefix(strings.ToUpper(createSQL[i:]), kw) {
		return createSQL
	}
	after := createSQL[i+len(kw):]
	upperAfter := strings.ToUpper(strings.TrimLeft(after, " \t\r\n"))
	if strings.HasPrefix(upperAfter, "TEMP") {
		return createSQL // already temp-qualified (TEMP or TEMPORARY)
	}
	return createSQL[:i+len(kw)] + " TEMP" + after
}

// splitTableQualifier splits a possibly schema-qualified table reference such as
// "main.user" or "temp.t" into its schema and object name. A bare name yields an
// empty schema. The split happens only when the prefix before the first dot is a
// real SQLite schema (main or temp); sqly rejects ATTACH/DETACH, so those are the
// only schemas a session can have. Any other dotted name (e.g. "a.b") is a literal
// table identifier and is returned whole, matching `SELECT * FROM "a.b"`. This is
// why `.schema "a.b"` reaches its table instead of querying a non-existent
// "a.sqlite_master".
func splitTableQualifier(name string) (schema, object string) {
	if i := strings.IndexByte(name, '.'); i > 0 && i < len(name)-1 && isSchemaName(name[:i]) {
		return name[:i], name[i+1:]
	}
	return "", name
}

// resolveObjectName disambiguates a user-supplied helper-command argument between
// a literal dotted identifier and a schema-qualified reference. The shell tokenizer
// strips the quotes the user typed, so "main.x" and main.x arrive identically and
// the name alone cannot say which was meant. It prefers the literal reading: when
// an object whose name is exactly tableName exists in the temp or main schema, the
// name is returned whole (empty schema), so `.schema "main.x"` reaches a table
// created as `CREATE TABLE "main.x"`. Otherwise a "main."/"temp."-prefixed name is
// split, so `.schema main.user` still resolves the imported user table.
func (s *Shell) resolveObjectName(ctx context.Context, tableName string) (schema, object string) {
	if s.objectExists(ctx, tableName) {
		return "", tableName
	}
	return splitTableQualifier(tableName)
}

// objectExists reports whether a table or view named exactly name exists in either
// the temp or main schema. A failed lookup reports false so resolution falls back
// to the syntactic schema-qualifier split.
func (s *Shell) objectExists(ctx context.Context, name string) bool {
	literal := "'" + strings.ReplaceAll(name, "'", "''") + "'"
	res, err := s.usecases.query.Query(ctx,
		"SELECT 1 FROM sqlite_temp_master WHERE name = "+literal+" AND type IN ('table', 'view') "+
			"UNION ALL SELECT 1 FROM sqlite_master WHERE name = "+literal+" AND type IN ('table', 'view')")
	if err != nil {
		return false
	}
	return len(res.Records()) > 0
}

// isSchemaName reports whether prefix is a SQLite schema name a sqly session can
// reference. Only "main" and "temp" (case-insensitive) qualify: ATTACH/DETACH are
// rejected, so no other database can be attached. A dotted prefix that is not one
// of these is therefore part of a literal table name, not a schema qualifier.
func isSchemaName(prefix string) bool {
	switch strings.ToLower(prefix) {
	case "main", "temp":
		return true
	default:
		return false
	}
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
