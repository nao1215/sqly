package shell

import (
	"context"
	"fmt"
	"io"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
)

// headerCommand print table header.
func (c CommandList) headerCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .header TABLE_NAME")
		return nil
	}

	table, err := s.sqlite3Interactor.Header(ctx, argv[0])
	if err != nil {
		return err
	}
	printHeader(config.Stdout, table)
	return nil
}

// printHeader print header
func printHeader(out io.Writer, t *model.Table) {
	table := tablewriter.NewWriter(out)
	table.SetHeader([]string{t.Name()})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(false)

	for _, v := range t.Header() {
		table.Append([]string{v})
	}
	table.Render()
}
