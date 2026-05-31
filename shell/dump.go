package shell

import (
	"context"
	"errors"
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
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .dump TABLE_NAME FILE_PATH")
		fmt.Fprintln(config.Stdout, "[Note]")
		fmt.Fprintln(config.Stdout, "  The format comes from .mode. When .mode is table, it is inferred from the")
		fmt.Fprintln(config.Stdout, "  file extension (e.g. .tsv, .parquet), falling back to CSV (written to the")
		fmt.Fprintln(config.Stdout, "  path as given) when the extension is unknown.")
		fmt.Fprintln(config.Stdout, "  Compression is inferred from the path (.gz, .xz, .zst, .z, .snappy, .s2, .lz4).")
		fmt.Fprintln(config.Stdout, "  A .mode that disagrees with the extension is rejected instead of normalizing.")
		fmt.Fprintln(config.Stdout, "  ACH/Fedwire tables can be dumped to csv/tsv/xlsx, but not back to .ach/.fed format.")
		return nil
	}

	tableName := argv[0]
	userPath := argv[1]

	// Reject an empty destination so `.dump table ""` does not write a file
	// named ".csv" into the current directory. Ref #324.
	if strings.TrimSpace(userPath) == "" {
		return errors.New(".dump requires a non-empty destination path")
	}

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

	// Reject a directory destination before doing any work, so it is not
	// silently rewritten to a sibling file (e.g. "dir" -> "dir.csv").
	if err := ensureNotDirectory(userPath); err != nil {
		return err
	}

	table, err := s.usecases.metadata.List(ctx, tableName)
	if err != nil {
		return err
	}

	// The current .mode sets the format unless it is table; otherwise the format
	// (and any compression) is inferred from the destination path.
	mode := s.state.mode.PrintMode
	exportFmt, compression, err := model.ResolveOutputTarget(userPath, model.ExportFormatFromPrintMode(mode), mode != model.PrintModeTable)
	if err != nil {
		return err
	}
	filePath := model.BuildOutputPath(userPath, exportFmt, compression)
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt, compression); err != nil {
		return err
	}
	// .dump writes data to a file, so its status line is control-plane output
	// and goes to stderr, keeping stdout free of non-data noise.
	fmt.Fprintf(config.Stderr, "dump `%s` table to %s (mode=%s)\n",
		color.CyanString(argv[0]), color.HiCyanString(filePath), exportFmt.String())

	return nil
}
