package shell

import (
	"context"
	"errors"
	"fmt"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// describeCommand prints one row per column of a table: ordinal position (cid),
// name, type, notnull, default value, and primary-key flag. The output respects
// the current output mode, so `.mode json` yields structured column metadata.
func (c CommandList) describeCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		// A missing required argument is a command error so a batch script fails
		// fast instead of skipping the command and exiting 0.
		return errors.New(".describe requires a table name\n[Usage]\n  .describe TABLE_NAME")
	}
	if len(argv) > 1 {
		return fmt.Errorf(".describe accepts a single table name, got %d arguments", len(argv))
	}
	tableName := argv[0]

	cols, err := s.tableColumns(ctx, tableName)
	if err != nil {
		return err
	}
	if len(cols.Records()) == 0 {
		return fmt.Errorf("no such table: %s", tableName)
	}
	return cols.Print(config.Stdout, s.state.mode.PrintMode)
}

// tableColumns returns the column metadata of a table via PRAGMA table_info.
// The result preserves definition order, giving stable column ordering. An
// empty record set means the table does not exist (PRAGMA returns no rows).
func (s *Shell) tableColumns(ctx context.Context, tableName string) (*model.Table, error) {
	// A schema-qualified name (main.user, temp.t) is inspected against that schema
	// via "PRAGMA schema.table_info(table)", matching the SQL surface. A literal
	// dotted name (a table created as "main.x") is resolved whole instead.
	schema, object := s.resolveObjectName(ctx, tableName)
	pragma := "PRAGMA "
	if schema != "" {
		pragma += s.usecases.importer.QuoteIdentifier(schema) + "."
	}
	query := pragma + "table_info(" + s.usecases.importer.QuoteIdentifier(object) + ")"
	table, err := s.usecases.query.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	// Re-wrap so the table carries the inspected name rather than the empty
	// name PRAGMA queries produce.
	return model.NewTable(tableName, table.Header(), table.Records()), nil
}
