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
	if len(argv) > 1 {
		return fmt.Errorf(".header accepts a single table name, got %d arguments", len(argv))
	}

	table, err := s.usecases.metadata.Header(ctx, argv[0])
	if err != nil {
		return err
	}

	// In a structured mode emit one machine-readable {column} object per column so
	// automation can consume the header after selecting json/ndjson, instead of an
	// ASCII table that ignores the mode.
	if isStructuredMode(s.state.mode.PrintMode) {
		records := make([]model.Record, 0, len(table.Header()))
		for _, col := range table.Header() {
			records = append(records, model.Record{col})
		}
		out := model.NewTable(table.Name(), model.Header{"column"}, records)
		return out.Print(config.Stdout, s.state.mode.PrintMode)
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
