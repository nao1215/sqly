package shell

import (
	"context"
	"fmt"
	"io"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
)

// headerCommand print table header.
func (c CommandList) headerCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .header TABLE_NAME")
		return nil
	}

	table, err := s.usecases.sqlite3.Header(ctx, argv[0])
	if err != nil {
		return err
	}
	if err := printHeader(config.Stdout, table); err != nil {
		return fmt.Errorf("failed to print header: %w", err)
	}
	return nil
}

// printHeader print header
func printHeader(out io.Writer, t *model.Table) error {
	table := tablewriter.NewTable(out,
		tablewriter.WithSymbols(tw.NewSymbols(tw.StyleASCII)),
		tablewriter.WithHeaderAutoFormat(tw.State(-1)),
		tablewriter.WithHeaderAlignmentConfig(tw.CellAlignment{Global: tw.AlignCenter}),
	)
	table.Header(t.Name())

	for _, v := range t.Header() {
		if err := table.Append([]any{v}); err != nil {
			return fmt.Errorf("failed to append header row: %w", err)
		}
	}
	if err := table.Render(); err != nil {
		return fmt.Errorf("failed to render header table: %w", err)
	}
	return nil
}
