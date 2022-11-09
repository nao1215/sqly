package shell

import (
	"fmt"
	"os"

	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
)

// headerCommand print table header.
func (c CommandList) headerCommand(s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(Stdout, "[Usage]")
		fmt.Fprintln(Stdout, "  .header TABLE_NAME")
		return nil
	}

	table, err := s.sqlite3Interactor.List(s.Ctx, argv[0])
	if err != nil {
		return err
	}
	table.Name = argv[0]
	printHeader(os.Stdout, table)
	return nil
}

// printHeader print header
func printHeader(out *os.File, t *model.Table) {
	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{t.Name})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)

	for _, v := range t.Header {
		table.Append([]string{v})
	}
	table.Render()
}
