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
	// named ".csv" into the current directory.
	if strings.TrimSpace(userPath) == "" {
		return errors.New(".dump requires a non-empty destination path")
	}

	// Block round-trip export to ACH/Fedwire format before normalization. These
	// formats require multi-record coordination that .dump cannot provide.
	// Exporting ACH/Fedwire tables to CSV/TSV/etc via .dump is fine. The check
	// strips any compression suffix, so .ach.gz and .fed.gz are rejected too.
	if model.IsInputOnlyExtension(userPath) {
		return fmt.Errorf(".dump does not support ACH/Fedwire format output; use csv/tsv/xlsx instead (e.g., .dump %s %s.csv)", tableName, strings.TrimSuffix(userPath, filepath.Ext(userPath)))
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
	// Carry the typed JSON contract into a JSON/NDJSON dump when the session is in
	// a typed mode; ignored for every other export format.
	table.SetJSONTyped(s.state.mode.jsonTyped)

	// The current .mode sets the format unless it is table; otherwise the format
	// (and any compression) is inferred from the destination path.
	mode := s.state.mode.PrintMode
	exportFmt, compression, err := model.ResolveOutputTarget(userPath, model.ExportFormatFromPrintMode(mode), mode != model.PrintModeTable)
	if err != nil {
		return err
	}
	filePath := model.BuildOutputPath(userPath, exportFmt, compression)
	// Refuse a destination that aliases an imported source file, including symlink
	// aliases. A destructive source overwrite must go through .save --force, not
	// .dump, so a stray .dump cannot silently rewrite the dataset in another
	// format.
	if name, aliased := s.outputAliasesImportedSource(filePath); aliased {
		return fmt.Errorf(".dump destination %s is the source file for table %q; use .save --force to overwrite a source", filePath, name)
	}
	if err := s.usecases.export.DumpTable(filePath, table, exportFmt, compression); err != nil {
		return err
	}
	// .dump writes data to a file, so its status line is control-plane output
	// and goes to stderr, keeping stdout free of non-data noise.
	fmt.Fprintf(config.Stderr, "dump `%s` table to %s (mode=%s)\n",
		color.CyanString(argv[0]), color.HiCyanString(filePath), exportFmt.String())

	return nil
}
