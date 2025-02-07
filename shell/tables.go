package shell

import (
	"context"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
)

// tablesCommand print all tables name in DB.
func (c CommandList) tablesCommand(ctx context.Context, s *Shell, _ []string) error {
	tables, err := s.usecases.sqlite3.TablesName(ctx)
	if err != nil {
		return err
	}
	printTables(config.Stdout, tables)
	return nil
}

// printTables print table name
func printTables(out io.Writer, t []*model.Table) {
	if len(t) == 0 {
		fmt.Fprintf(config.Stdout,
			"there is no table. use %s for importing file\n",
			color.CyanString(".import"))
		return
	}

	tableData := [][]string{}
	for _, v := range t {
		tableData = append(tableData, []string{v.Name()})
	}

	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"table_name"})
	table.SetAutoWrapText(false)

	for _, v := range tableData {
		table.Append(v)
	}
	table.Render()
}
