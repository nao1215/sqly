package shell

import (
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
)

// tablesCommand print all tables name in DB.
func (c CommandList) tablesCommand(s *Shell, argv []string) error {
	tables, err := s.sqlite3Interactor.TablesName(s.Ctx)
	if err != nil {
		return err
	}
	printTables(os.Stdout, tables)
	return nil
}

// printTables print table name
func printTables(out *os.File, t []*model.Table) {
	tableData := [][]string{}
	for _, v := range t {
		tableData = append(tableData, []string{v.Name})
	}

	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{"table_name"})
	table.SetAutoWrapText(false)

	for _, v := range tableData {
		table.Append(v)
	}
	table.Render()
}
