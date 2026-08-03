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

// CreateTable create a DB table with columns given as model.Table
func (r *sqlite3Repository) CreateTable(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	return r.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, infra.GenerateCreateTableStatement((t)))
		return err
	})
}

// TablesName return all table name in import order.
// Internal tables (sqlite_* and query_result_*) are excluded from the result.
// Rows are ordered by sqlite_master.rowid, which is assigned in CREATE order, so
// the result follows the order the source files were imported.
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	tables := []*model.Table{}
	err := r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' ORDER BY rowid")
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
// file-imported base tables that write-back targets. Internal bookkeeping tables
// (sqlite_* and query_result_*) are excluded, and names are sorted for stable
// output.
//
// Each returned table carries the raw object name in Name() and the owning schema
// ("main" or "temp") as the single Header entry, so .tables can disambiguate a
// main object and a same-named temp object instead of collapsing them. UNION ALL
// (not UNION) keeps both rows of such a collision.
func (r *sqlite3Repository) SchemaObjects(ctx context.Context) ([]*model.Table, error) {
	const query = "SELECT name, 'main' AS schema_name FROM sqlite_master " +
		"WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' " +
		"UNION ALL " +
		"SELECT name, 'temp' AS schema_name FROM sqlite_temp_master " +
		"WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' " +
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

// Insert set records in DB
func (r *sqlite3Repository) Insert(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	return r.inTx(ctx, func(tx *sql.Tx) error {
		for _, v := range t.Rows {
			if _, err := tx.ExecContext(ctx, infra.GenerateInsertStatement(t.Name(), v)); err != nil {
				return err
			}
		}
		return nil
	})
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

// QueryStream executes a read query and invokes fn once per result row, scanning
// rows one at a time so a caller can aggregate without holding the whole result
// set in memory. Each call gets the row's cell strings and a per-cell SQL NULL
// flag (distinguished the same way Query does, via the driver's native value).
func (r *sqlite3Repository) QueryStream(ctx context.Context, query string, fn func(record []string, nulls []bool) error) error {
	return r.inTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()

		header, err := rows.Columns()
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
			record := make([]string, len(header))
			nulls := make([]bool, len(header))
			for i, value := range values {
				cell := model.NewCell(value)
				nulls[i] = cell.IsNull()
				record[i] = cell.String()
			}
			if err := fn(record, nulls); err != nil {
				return err
			}
		}
		return rows.Err()
	})
}

// extractTableName extract table name from query.
// The query must be "SELECT" or "EXPLAIN" statement.
func extractTableName(query string) string {
	query = strings.ReplaceAll(query, "`", "")
	words := strings.Fields(query)
	for i, v := range words {
		if strings.EqualFold(v, "FROM") || strings.EqualFold(v, "from") {
			return words[i+1]
		}
	}
	return ""
}

// Exec execute "INSERT" or "UPDATE" or "DELETE" statement
func (r *sqlite3Repository) Exec(ctx context.Context, statement string) (int64, error) {
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
