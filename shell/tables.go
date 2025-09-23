package shell

import (
	"context"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// tablesCommand print all tables name in DB.
func (c CommandList) tablesCommand(ctx context.Context, s *Shell, _ []string) error {
	tables, err := s.usecases.sqlite3.TablesName(ctx)
	if err != nil {
		return err
	}
	if err := printTables(config.Stdout, tables); err != nil {
		return fmt.Errorf("failed to print tables: %w", err)
	}
	return nil
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
		tableData = append(tableData, []string{v.Name()})
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
