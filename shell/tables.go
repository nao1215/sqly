package shell

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// Column keys for the structured (.mode json/ndjson) .tables listing.
const (
	tablesColumnName   = "name"
	tablesColumnSchema = "schema"
)

// tablesCommand print all tables name in DB.
func (c CommandList) tablesCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) > 0 {
		return fmt.Errorf(".tables takes no arguments, got %d", len(argv))
	}
	// List every queryable object (base tables, views, and TEMP tables/views),
	// not only the file-imported base tables, so a session-created view or temp
	// table is discoverable here the same way it is queryable.
	tables, err := s.usecases.metadata.SchemaObjects(ctx)
	if err != nil {
		return err
	}

	// In a structured mode emit one machine-readable {name, schema} object per
	// table, so the schema disambiguates a main object from a same-named temp
	// object and automation can consume the list.
	if isStructuredMode(s.state.mode.PrintMode) {
		records := make([]model.Record, 0, len(tables))
		for _, t := range tables {
			records = append(records, model.Record{t.Name(), schemaOf(t)})
		}
		out := model.NewTable("tables", model.Header{tablesColumnName, tablesColumnSchema}, records)
		return out.Print(config.Stdout, s.state.mode.PrintMode)
	}

	if err := printTables(config.Stdout, tables); err != nil {
		return fmt.Errorf("failed to print tables: %w", err)
	}
	return nil
}

// schemaOf returns the owning schema ("main" or "temp") that SchemaObjects encoded
// as a table's single Header entry, or "main" when absent.
func schemaOf(t *model.Table) string {
	if h := t.Header(); len(h) > 0 && h[0] != "" {
		return h[0]
	}
	return "main"
}

// tableDisplayName renders an object name for the ASCII .tables listing so it can
// be pasted straight back into SQL or a helper command: identifiers that require
// quoting are double-quoted, and a temp object is qualified with "temp." to keep
// it distinct from a same-named main object.
func tableDisplayName(t *model.Table) string {
	name := quoteIdentifierIfNeeded(t.Name())
	if schemaOf(t) == "temp" {
		return "temp." + name
	}
	return name
}

// quoteIdentifierIfNeeded returns name unchanged when it is a safe bare SQL
// identifier, and a double-quoted form (inner quotes doubled) otherwise, so the
// result is always safe to paste back into SQL.
func quoteIdentifierIfNeeded(name string) string {
	if isBareIdentifier(name) && !model.IsReservedSQLiteKeyword(name) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// isBareIdentifier reports whether s is a plain SQL identifier that needs no
// quoting: it starts with a letter or underscore and contains only letters,
// digits, and underscores.
func isBareIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// printTables print table name
func printTables(out io.Writer, t []*model.Table) error {
	if len(t) == 0 {
		fmt.Fprintf(out,
			"there is no table. use %s for importing file\n",
			color.CyanString(".import"))
		return nil
	}

	tableData := [][]string{}
	for _, v := range t {
		tableData = append(tableData, []string{tableDisplayName(v)})
	}

	table := tablewriter.NewTable(out,
		tablewriter.WithSymbols(tw.NewSymbols(tw.StyleASCII)),
		tablewriter.WithHeaderAutoFormat(tw.State(-1)),
		tablewriter.WithHeaderAlignmentConfig(tw.CellAlignment{Global: tw.AlignCenter}),
	)
	table.Header("TABLE NAME")

	for _, v := range tableData {
		// Convert []string to []any for the new API
		row := make([]any, len(v))
		for i, cell := range v {
			row[i] = cell
		}
		if err := table.Append(row); err != nil {
			return fmt.Errorf("failed to append table row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("failed to render tables list: %w", err)
	}
	return nil
}
