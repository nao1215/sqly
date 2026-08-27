// Package memory handle sqlite3 in memory mode
package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/nao1215/sqly/domain/repository"
	"github.com/nao1215/sqly/domain/sqltext"
	infra "github.com/nao1215/sqly/infrastructure"
)

type sqlite3Repository struct {
	db *sql.DB
}

// NewSQLite3Repository return sqlite3Repository
func NewSQLite3Repository(db config.MemoryDB) repository.SQLite3Repository {
	return &sqlite3Repository{db: db}
}

// inTx runs fn inside a transaction on the session database, delegating the
// commit, the rollback, and the reporting of a cleanup failure to the single
// implementation in the infrastructure package. Every method here used to spell
// that out itself as a deferred rollback whose error was discarded, which meant
// eight places could drift apart and none of them could report a rollback that
// failed. The success path commits and never rolls back afterwards, so a
// sql.ErrTxDone from a rollback is a real defect rather than the expected
// no-op it used to be.
func (r *sqlite3Repository) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	_, err := infra.WithTransaction(ctx, infra.SQLTxBeginner{DB: r.db}, fn)
	return err
}

// TablesName return all table name in import order.
// SQLite's own bookkeeping tables (sqlite_sequence, sqlite_stat1) are excluded,
// as is the one filesql keeps to remember where an ACH or Fedwire table was
// loaded from. Each prefix is reserved by the layer that owns it: SQLite refuses
// to create a table under sqlite_, and filesql refuses an import whose table
// name would fall under _filesql_. Nothing a user imports can therefore be
// hidden by the exclusions.
// Rows are ordered by sqlite_master.rowid, which is assigned in CREATE order, so
// the result follows the order the source files were imported.
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	tables := []*model.Table{}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table'"+
				" AND name NOT LIKE 'sqlite_%'"+
				` AND name NOT LIKE '\_filesql\_%' ESCAPE '\'`+
				" ORDER BY rowid")
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		var name string
		for rows.Next() {
			if err := rows.Scan(&name); err != nil {
				return err
			}
			tables = append(tables, model.NewTable(name, model.Header{}, []model.Record{}))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return tables, nil
}

// SchemaObjects returns every queryable table and view in the session: base
// tables and views in the main schema plus TEMP tables and views. It backs
// .tables, which should enumerate everything the user can query, not only the
// file-imported base tables that write-back targets. The reserved sqlite_ and
// _filesql_ prefixes are excluded: no import can produce a name under either, so
// the exclusion hides only the two layers' own bookkeeping. Names are sorted for
// stable output.
//
// Each returned table carries the raw object name in Name() and the owning schema
// ("main" or "temp") as the single Header entry, so .tables can disambiguate a
// main object and a same-named temp object instead of collapsing them. UNION ALL
// (not UNION) keeps both rows of such a collision.
func (r *sqlite3Repository) SchemaObjects(ctx context.Context) ([]*model.Table, error) {
	const reserved = ` AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '\_filesql\_%' ESCAPE '\' `
	const query = "SELECT name, 'main' AS schema_name FROM sqlite_master " +
		"WHERE type IN ('table', 'view')" + reserved +
		"UNION ALL " +
		"SELECT name, 'temp' AS schema_name FROM sqlite_temp_master " +
		"WHERE type IN ('table', 'view')" + reserved +
		"ORDER BY name"

	tables := []*model.Table{}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		var name, schemaName string
		for rows.Next() {
			if err := rows.Scan(&name, &schemaName); err != nil {
				return err
			}
			tables = append(tables, model.NewTable(name, model.Header{schemaName}, []model.Record{}))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return tables, nil
}

// List get records in the specified table
func (r *sqlite3Repository) List(ctx context.Context, tableName string) (*model.Table, error) {
	ref, err := r.resolveTableRef(ctx, tableName)
	if err != nil {
		return nil, err
	}
	table, err := r.Query(ctx, "SELECT * FROM "+ref)
	if err != nil {
		return nil, err
	}
	return table.WithName(tableName), nil
}

// Header get table header name. The result is re-wrapped with the requested table
// name rather than the name extractTableName parses from the query, which would
// truncate a name containing spaces (e.g. "two words" -> "two").
func (r *sqlite3Repository) Header(ctx context.Context, tableName string) (*model.Table, error) {
	ref, err := r.resolveTableRef(ctx, tableName)
	if err != nil {
		return nil, err
	}
	table, err := r.Query(ctx, fmt.Sprintf("SELECT * FROM %s LIMIT 1", ref))
	if err != nil {
		return nil, err
	}
	return table.WithName(tableName), nil
}

// resolveTableRef returns the quoted SQL reference a helper command should query
// for tableName, disambiguating a literal dotted name from a schema-qualified one.
// It prefers the literal reading: when an object whose name is exactly tableName
// exists, the name is quoted whole (so `.dump "main.x"` reaches a table created as
// `CREATE TABLE "main.x"`); otherwise a "main."/"temp."-prefixed name keeps its
// schema-qualified quoting (so `.dump main.user` resolves the imported user table).
func (r *sqlite3Repository) resolveTableRef(ctx context.Context, tableName string) (string, error) {
	exists, err := r.objectExists(ctx, tableName)
	if err != nil {
		return "", err
	}
	if exists {
		return infra.Quote(tableName), nil
	}
	return infra.QuoteTableRef(tableName), nil
}

// objectExists reports whether a table or view whose name is exactly name exists
// in either the temp or main schema.
func (r *sqlite3Repository) objectExists(ctx context.Context, name string) (bool, error) {
	const query = "SELECT 1 FROM sqlite_temp_master WHERE name = ? AND type IN ('table', 'view') " +
		"UNION ALL SELECT 1 FROM sqlite_master WHERE name = ? AND type IN ('table', 'view') LIMIT 1"

	exists := false
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		var dummy int
		err := tx.QueryRowContext(ctx, query, name, name).Scan(&dummy)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Query execute "SELECT" or "EXPLAIN" query
func (r *sqlite3Repository) Query(ctx context.Context, query string) (*model.Table, error) {
	var header []string
	// Each row is kept as the driver's native cells. model.Cell derives the
	// display string from that same value, so the strings the table and CSV
	// formats print and the scalars the JSON formats emit cannot disagree, and a
	// SQL NULL stays distinguishable from an empty string.
	cells := [][]model.Cell{}

	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		header, err = rows.Columns()
		if err != nil {
			return err
		}
		if len(header) == 0 {
			return repository.ErrNoRows
		}

		scanDest := make([]any, len(header))
		values := make([]any, len(header))
		for i := range header {
			scanDest[i] = &values[i]
		}

		for rows.Next() {
			if err := rows.Scan(scanDest...); err != nil {
				return err
			}
			row := make([]model.Cell, len(header))
			for i, value := range values {
				row[i] = model.NewCell(value)
			}
			cells = append(cells, row)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return model.NewTableFromCells(extractTableName(query), header, cells)
}

// extractTableName returns the object named by the query's first FROM clause. It
// names the result table, which is what an Excel export uses as its worksheet
// name, so a query with no FROM (or none that names anything) yields "" and the
// writer falls back to its own name.
//
// The FROM is found through sqltext rather than by splitting the query into
// words. A word split cannot tell code from the rest: it read the "from" of a
// trailing comment, of a string literal, and of a column aliased `from` as the
// clause, and then read the word after it — which, when that "from" was the last
// word, was past the end of the list.
func extractTableName(query string) string {
	for token := range sqltext.Tokens(query) {
		if token.Kind == sqltext.Word && strings.EqualFold(token.Text(query), "FROM") {
			return firstIdentifier(query[token.End:])
		}
	}
	return ""
}

// firstIdentifier returns the identifier that opens s, with one layer of SQLite
// quoting removed (`x`, "x", [x]). A schema qualifier stays part of the name, so
// "main.people" is reported whole.
//
// It returns "" when s opens with something that is not a name — most often the
// "(" of a subquery, whose SELECT is not a table anyone can refer to.
func firstIdentifier(s string) string {
	s = strings.TrimLeft(s, " \t\r\n")
	if s == "" {
		return ""
	}
	if closer, quoted := identifierQuotes[s[0]]; quoted {
		end := strings.IndexByte(s[1:], closer)
		if end < 0 {
			return "" // the quote never closes, so there is no name to read
		}
		return s[1 : 1+end]
	}
	end := 0
	for end < len(s) && isIdentifierByte(s[end]) {
		end++
	}
	return s[:end]
}

// identifierQuotes maps each character that opens a quoted identifier in SQLite
// to the one that closes it.
var identifierQuotes = map[byte]byte{'`': '`', '"': '"', '[': ']'}

// isIdentifierByte reports whether c can be part of an unquoted table reference.
// "." is included so a schema-qualified name is read as one.
func isIdentifierByte(c byte) bool {
	return c == '_' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
//
// A statement with nothing executable in it — empty, whitespace, a bare ";", a
// comment — affects no rows and is not sent to the driver. The driver answers
// such a statement with a result that carries no underlying result, and reading a
// row count off that crashes rather than returning an error. Callers reach here
// with one when a dialect translation removes the only thing the statement held.
func (r *sqlite3Repository) Exec(ctx context.Context, statement string) (int64, error) {
	if sqltext.StripNoise(statement) == "" {
		return 0, nil
	}

	var result sql.Result
	if err := r.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		result, err = tx.ExecContext(ctx, statement)
		return err
	}); err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
