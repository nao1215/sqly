package shell

import (
	"context"
	"fmt"
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
	filePath := normalizeDumpExt(argv[1], exportFmt)
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt); err != nil {
		return err
	}
	fmt.Fprintf(config.Stdout, "dump `%s` table to %s (mode=%s)\n",
		color.CyanString(argv[0]), color.HiCyanString(filePath), exportFmt.String())

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
