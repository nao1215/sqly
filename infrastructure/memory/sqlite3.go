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

// CreateTable create a DB table with columns given as model.Table
func (r *sqlite3Repository) CreateTable(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, infra.GenerateCreateTableStatement((t)))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// TablesName return all table name in import order.
// Internal tables (sqlite_* and query_result_*) are excluded from the result.
// Rows are ordered by sqlite_master.rowid, which is assigned in CREATE order, so
// the result follows the order the source files were imported. Callers such as
// --compare rely on this to keep left/right matching the CLI input order.
func (r *sqlite3Repository) TablesName(ctx context.Context) ([]*model.Table, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx,
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tables := []*model.Table{}
	var name string
	for rows.Next() {
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, model.NewTable(name, model.Header{}, []model.Record{}))
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	const query = "SELECT name, 'main' AS schema_name FROM sqlite_master " +
		"WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' " +
		"UNION ALL " +
		"SELECT name, 'temp' AS schema_name FROM sqlite_temp_master " +
		"WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'query_result_%' " +
		"ORDER BY name"
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tables := []*model.Table{}
	var name, schemaName string
	for rows.Next() {
		if err := rows.Scan(&name, &schemaName); err != nil {
			return nil, err
		}
		tables = append(tables, model.NewTable(name, model.Header{schemaName}, []model.Record{}))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return tables, nil
}

// Insert set records in DB
func (r *sqlite3Repository) Insert(ctx context.Context, t *model.Table) error {
	if err := t.Valid(); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, v := range t.Records() {
		if _, err := tx.ExecContext(ctx, infra.GenerateInsertStatement(t.Name(), v)); err != nil {
			return err
		}
	}
	return tx.Commit()
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
	return model.NewTable(tableName, table.Header(), table.Records()), nil
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
	return model.NewTable(tableName, table.Header(), table.Records()), nil
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	const query = "SELECT 1 FROM sqlite_temp_master WHERE name = ? AND type IN ('table', 'view') " +
		"UNION ALL SELECT 1 FROM sqlite_master WHERE name = ? AND type IN ('table', 'view') LIMIT 1"
	var dummy int
	err = tx.QueryRowContext(ctx, query, name, name).Scan(&dummy)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// Query execute "SELECT" or "EXPLAIN" query
func (r *sqlite3Repository) Query(ctx context.Context, query string) (*model.Table, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	header, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(header) == 0 {
		return nil, repository.ErrNoRows
	}

	scanDest := make([]any, len(header))
	rawResult := make([][]byte, len(header))
	for i := range header {
		scanDest[i] = &rawResult[i]
	}

	records := []model.Record{}
	// nulls tracks which cells were SQL NULL. With a *[]byte scan target a NULL
	// yields a nil slice, while an empty string yields a non-nil empty slice, so
	// the two are distinguishable here even though both render as "".
	nulls := [][]bool{}
	for rows.Next() {
		result := make([]string, len(header))
		rowNulls := make([]bool, len(header))
		err := rows.Scan(scanDest...)
		if err != nil {
			return nil, err
		}

		for i, raw := range rawResult {
			if raw == nil {
				rowNulls[i] = true
				continue
			}
			result[i] = string(raw)
		}
		records = append(records, result)
		nulls = append(nulls, rowNulls)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	table := model.NewTable(extractTableName(query), header, records)
	table.SetNulls(nulls)
	return table, nil
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, statement)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
