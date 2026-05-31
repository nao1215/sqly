package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// forceArg is the .save argument that selects destructive in-place overwrite.
const forceArg = "--force"

// saveCommand writes the current tables back to files from the interactive
// shell. ".save DIR" writes into a directory without touching the sources;
// ".save --force" overwrites the source files in place.
func (c CommandList) saveCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) != 1 {
		fmt.Fprintln(config.Stdout, "[Usage]")
		fmt.Fprintln(config.Stdout, "  .save DIRECTORY   write each table into DIRECTORY (originals untouched)")
		fmt.Fprintln(config.Stdout, "  .save --force     overwrite each table's source file in place")
		fmt.Fprintln(config.Stdout, "[Note]")
		fmt.Fprintln(config.Stdout, "  Only csv/tsv/ltsv/parquet sources are written; compression is preserved.")
		return nil
	}
	if argv[0] == forceArg {
		return s.writeBack(ctx, "")
	}
	// Reject an empty destination so `.save ""` is not treated as an in-place
	// save, which would bypass the --force safeguard. Ref #323.
	if strings.TrimSpace(argv[0]) == "" {
		return errors.New(".save requires a non-empty directory; use .save --force to overwrite sources in place")
	}
	return s.writeBack(ctx, argv[0])
}

// validateSaveFlags checks the --save/--save-dir/--force combination before any
// work runs. In-place overwrite must be confirmed with --force, the two save
// destinations are mutually exclusive, and the save flags only apply to
// non-interactive runs (the interactive shell uses the .save command).
func (s *Shell) validateSaveFlags() error {
	if !s.argument.SaveInPlace && s.argument.SaveDir == "" {
		return nil
	}
	if s.argument.SaveInPlace && s.argument.SaveDir != "" {
		return errors.New("--save and --save-dir cannot be used together")
	}
	if s.argument.SaveInPlace && !s.argument.Force {
		return errors.New("--save overwrites source files; pass --force to confirm, or use --save-dir DIR to write elsewhere")
	}
	// --sql-file is a non-interactive execution path just like --sql and piped
	// input, so it may carry write-back even when stdin is a TTY. Reject the save
	// flags only for a genuinely interactive run with no query source. Ref #366,
	// #367.
	if s.argument.Query == "" && s.argument.SQLFilePath == "" && s.isTTY() {
		return errors.New("--save/--save-dir require --sql, --sql-file, or piped input; use the .save command in the interactive shell")
	}
	return nil
}

// maybeSave runs write-back after a non-interactive run when a save flag is set.
func (s *Shell) maybeSave(ctx context.Context) error {
	switch {
	case s.argument.SaveDir != "":
		return s.writeBack(ctx, s.argument.SaveDir)
	case s.argument.SaveInPlace:
		return s.writeBack(ctx, "")
	default:
		return nil
	}
}

// writeBack persists the current tables to files. When destDir is empty the
// tables are written back over their source files in place (destructive); the
// caller must have confirmed --force. When destDir is set the tables are written
// into that directory, preserving each source's file name, and the original
// source files are left untouched.
//
// Only tables that map 1:1 to a single editable source file are written:
// CSV, TSV, LTSV, and Parquet, with the source's compression preserved. Tables
// without a file source (created by SQL), tables from a directory import, tables
// that share a source with others (Excel sheets, ACH/Fedwire), and unsupported
// formats are rejected with a clear error before anything is written, so a
// session is never partially saved.
func (s *Shell) writeBack(ctx context.Context, destDir string) error {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		return errors.New("no tables to save")
	}

	// Count how many tables map to each source so multi-table sources (Excel,
	// ACH/Fedwire) can be rejected.
	tablesPerSource := make(map[string]int)
	for _, t := range tables {
		if src, ok := s.tableSources[t.Name()]; ok {
			tablesPerSource[src]++
		}
	}

	type target struct {
		table  string
		dest   string
		format model.ExportFormat
		comp   model.Compression
	}
	var targets []target
	var problems []string
	// Detect destination collisions so two tables never silently overwrite the
	// same output file (defensive: same-name files already collapse to one table
	// at import, but a future rename could break that assumption).
	plannedDest := make(map[string]string)

	for _, t := range tables {
		name := t.Name()
		source, ok := s.tableSources[name]
		if !ok {
			problems = append(problems, name+": not loaded from a file")
			continue
		}
		if source == stdinTableSource {
			problems = append(problems, name+": came from --stdin and has no source file to write back to")
			continue
		}
		// A directory import is not a single editable source the session owns, so
		// reject it even though its source may point at a per-file path for
		// --inspect provenance. Ref #326, #261.
		if s.dirImported[name] {
			problems = append(problems, fmt.Sprintf("%s: came from a directory import (%s)", name, source))
			continue
		}
		if info, statErr := os.Stat(source); statErr == nil && info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s: came from a directory import (%s)", name, source))
			continue
		}
		if tablesPerSource[source] > 1 {
			problems = append(problems, fmt.Sprintf("%s: shares source %s with other tables (Excel/ACH/Fedwire)", name, source))
			continue
		}
		format, comp, supported := writableExportTarget(source)
		if !supported {
			problems = append(problems, fmt.Sprintf("%s: write-back to %s is not supported (use csv, tsv, ltsv, or parquet)", name, filepath.Base(source)))
			continue
		}

		dest := source
		if destDir != "" {
			dest = filepath.Join(destDir, filepath.Base(source))
		}
		if prev, ok := plannedDest[dest]; ok {
			problems = append(problems, fmt.Sprintf("%s: destination %s collides with table %s", name, dest, prev))
			continue
		}
		plannedDest[dest] = name
		targets = append(targets, target{table: name, dest: dest, format: format, comp: comp})
	}

	if len(problems) > 0 {
		return fmt.Errorf("cannot save session:\n  - %s", strings.Join(problems, "\n  - "))
	}

	if destDir != "" {
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return fmt.Errorf("failed to create save directory %q: %w", destDir, err)
		}
	}

	for _, tgt := range targets {
		table, err := s.usecases.metadata.List(ctx, tgt.table)
		if err != nil {
			return fmt.Errorf("failed to read table %s: %w", tgt.table, err)
		}
		if err := s.usecases.export.DumpTable(tgt.dest, table, tgt.format, tgt.comp); err != nil {
			return fmt.Errorf("failed to save table %s to %s: %w", tgt.table, tgt.dest, err)
		}
		// Write-back is a file-output operation; its confirmation is control-plane
		// output and goes to stderr so stdout stays free of non-data noise.
		fmt.Fprintf(config.Stderr, "Saved %s to %s\n", tgt.table, tgt.dest)
	}
	return nil
}

// writableExportTarget reports whether a source path can be written back, and
// the export format and compression to use. Only the formats that round-trip
// cleanly through sqly's table model are allowed: CSV, TSV, LTSV (with the
// source's compression), and Parquet. JSON/JSONL (stored in a single data
// column), Excel, ACH, and Fedwire are not.
func writableExportTarget(source string) (model.ExportFormat, model.Compression, bool) {
	comp := model.CompressionNone
	base := source
	if c, ok := model.CompressionFromExtension(filepath.Ext(source)); ok {
		comp = c
		base = strings.TrimSuffix(source, filepath.Ext(source))
	}
	format, ok := model.ExportFormatFromExtension(filepath.Ext(base))
	if !ok {
		return 0, model.CompressionNone, false
	}
	switch format {
	case model.ExportCSV, model.ExportTSV, model.ExportLTSV:
		return format, comp, true
	case model.ExportParquet:
		if comp != model.CompressionNone {
			return 0, model.CompressionNone, false
		}
		return format, model.CompressionNone, true
	default:
		return 0, model.CompressionNone, false
	}
}
