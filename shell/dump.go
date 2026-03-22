package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// dumpCommand dump specified table to file in the current export format
func (c CommandList) dumpCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) != 2 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .dump TABLE_NAME FILE_PATH")
		fmt.Fprintln(config.Stdout, "[Note]")
		fmt.Fprintln(config.Stdout, "  Output will be in the format specified in .mode.")
		fmt.Fprintln(config.Stdout, "  table mode is not available in .dump. If mode is table, .dump output CSV file.")
		return nil
	}

	table, err := s.usecases.sqlite3.List(ctx, argv[0])
	if err != nil {
		return err
	}

	exportFmt := model.ExportFormatFromPrintMode(s.state.mode.PrintMode)
	if err := dumpToFile(s, argv[1], table, exportFmt); err != nil {
		return err
	}
	fmt.Fprintf(config.Stdout, "dump `%s` table to %s (mode=%s)\n",
		color.CyanString(argv[0]), color.HiCyanString(argv[1]), exportFmt.String())

	return nil
}

// normalizeDumpExt normalizes the output file extension based on the export format
func normalizeDumpExt(path string, ef model.ExportFormat) string {
	ext := ef.Extension()
	if filepath.Ext(path) == ext {
		return path
	}
	return strings.TrimSuffix(path, filepath.Ext(path)) + ext
}

// dumpToFile writes table data to file using the specified export format
func dumpToFile(s *Shell, filePath string, table *model.Table, ef model.ExportFormat) error {
	filePath = normalizeDumpExt(filePath, ef)
	switch ef {
	case model.ExportCSV:
		return s.usecases.csv.Dump(filePath, table)
	case model.ExportTSV:
		return s.usecases.tsv.Dump(filePath, table)
	case model.ExportLTSV:
		return s.usecases.ltsv.Dump(filePath, table)
	case model.ExportExcel:
		return s.usecases.excel.Dump(filePath, table)
	case model.ExportMarkdown:
		return dumpMarkdown(filePath, table)
	default:
		return s.usecases.csv.Dump(filePath, table)
	}
}

// dumpMarkdown writes table data to file in Markdown table format
func dumpMarkdown(filePath string, table *model.Table) error {
	f, err := os.Create(filePath) // #nosec G304 - path is validated by normalizeDumpExt
	if err != nil {
		return fmt.Errorf("failed to create markdown file %s: %w", filePath, err)
	}
	defer f.Close()

	return table.Print(f, model.PrintModeMarkdownTable)
}
