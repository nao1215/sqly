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
	const expectedArgLen = 2
	if len(argv) != expectedArgLen {
		_, _ = fmt.Fprintln(config.Stdout, "[Usage]")
		_, _ = fmt.Fprintln(config.Stdout, "  .dump TABLE_NAME FILE_PATH")
		_, _ = fmt.Fprintln(config.Stdout, "[Note]")
		_, _ = fmt.Fprintln(config.Stdout, "  Output will be in the format specified in .mode.")
		_, _ = fmt.Fprintln(config.Stdout, "  table mode is not available in .dump. If mode is table, .dump output CSV file.")
		_, _ = fmt.Fprintln(config.Stdout, "  ACH/Fedwire tables can be dumped to csv/tsv/xlsx, but not back to .ach/.fed format.")
		return nil
	}

	tableName := argv[0]
	userPath := argv[1]

	// Block round-trip export to .ach/.fed format before normalization.
	// These formats require multi-table coordination that .dump cannot provide.
	// Exporting ACH/Fedwire tables to CSV/TSV/etc via .dump is fine.
	lowerUserPath := strings.ToLower(userPath)
	if strings.HasSuffix(lowerUserPath, ".ach") {
		return fmt.Errorf(".dump does not support ACH format output; use csv/tsv/xlsx instead (e.g., .dump %s %s.csv)", tableName, strings.TrimSuffix(userPath, filepath.Ext(userPath)))
	}
	if strings.HasSuffix(lowerUserPath, ".fed") {
		return fmt.Errorf(".dump does not support Fedwire format output; use csv/tsv/xlsx instead (e.g., .dump %s %s.csv)", tableName, strings.TrimSuffix(userPath, filepath.Ext(userPath)))
	}

	table, err := s.usecases.sqlite3.List(ctx, tableName)
	if err != nil {
		return err
	}

	exportFmt := model.ExportFormatFromPrintMode(s.state.mode.PrintMode)
	filePath := normalizeDumpExt(userPath, exportFmt)
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(config.Stdout, "dump `%s` table to %s (mode=%s)\n",
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
