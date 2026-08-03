package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// inPlaceArg is the .save argument that selects destructive in-place overwrite.
// It matches the --save-in-place flag so the shell and the command line describe
// the same operation with the same word.
const inPlaceArg = "--in-place"

// noDataChangedMessage explains a save that wrote nothing because the run left
// every table as imported. Both the .save command and the non-interactive
// --save-in-place/--save-tables path report it, so a read-only run never looks like a
// successful write-back that silently produced no file.
const noDataChangedMessage = "no table data changed in this session; nothing to save"

// saveCommand writes the current tables back to files from the interactive
// shell. ".save DIR" writes into a directory without touching the sources;
// ".save --in-place" overwrites the source files.
func (c CommandList) saveCommand(ctx context.Context, s *Shell, argv []string) error {
	if len(argv) != 1 {
		// A missing or extra argument is a command error so a batch script fails
		// fast instead of skipping the save and exiting 0. The usage and note ride
		// on the error path.
		return errors.New(".save requires a single argument: a directory or --in-place\n" +
			"[Usage]\n" +
			"  .save DIRECTORY   write each table into DIRECTORY (originals untouched)\n" +
			"  .save --in-place  overwrite each table's source file\n" +
			"[Note]\n" +
			"  csv/tsv/ltsv/parquet sources are written; compression is preserved.\n" +
			"  A whole ACH/Fedwire set is reconstructed back into a single .ach/.fed file\n" +
			"  when all of that source's tables are still present")
	}
	// Reject an empty destination so `.save ""` is not silently treated as an
	// in-place save, which the user never asked for.
	if argv[0] != inPlaceArg && strings.TrimSpace(argv[0]) == "" {
		return errors.New(".save requires a non-empty directory; use .save --in-place to overwrite the sources")
	}
	// Anything else beginning with "-" is a flag the user meant, not a directory
	// they want created. Taking it as a destination made `.save --force` — the
	// spelling this command used before --in-place — silently write a directory
	// named "--force" instead of overwriting the sources it was asked to.
	if argv[0] != inPlaceArg && strings.HasPrefix(argv[0], "-") {
		return fmt.Errorf(".save does not have a %s option; write to a directory with .save DIR, or overwrite the sources with .save %s", argv[0], inPlaceArg)
	}
	// An empty session has no tables at all (forgot to .import, or a prior import
	// failed), which is a different mistake from a read-only session below. Save is
	// safety-sensitive, so guide the user to load a table instead of emitting a
	// bare no-op.
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		return noTablesToSaveError(s.isTTY())
	}
	// A read-only session changed no table data, so there is nothing to persist.
	// Writing here would rewrite source files (or emit fresh directory exports)
	// with no logical change, normalizing bytes (e.g. the trailing newline) and
	// producing surprising diffs and checksum churn. This mirrors the
	// non-interactive write-back contract, which also skips write-back for
	// a read-only run.
	if !s.dataChanged {
		fmt.Fprintln(config.Stderr, noDataChangedMessage)
		return nil
	}
	if argv[0] == inPlaceArg {
		return s.writeBack(ctx, "")
	}
	// Expand a leading "~" so `.save ~/out` writes under the home directory
	// instead of a literal "~" directory.
	destDir, err := expandTilde(argv[0])
	if err != nil {
		return err
	}
	return s.writeBack(ctx, destDir)
}

// noTablesToSaveError builds the empty-session save error with recovery guidance
// tailored to the run mode. Save is safety-sensitive, so the message names the
// next step (.import a file, or pass input files) instead of a bare "no tables to
// save".
func noTablesToSaveError(interactive bool) error {
	if interactive {
		return errors.New("no tables to save: run .import FILE to load a table first")
	}
	return errors.New("no tables to save: pass input files (e.g. sqly data.csv ...) before saving")
}

// validateSaveFlags checks the write-back flags before anything is imported.
// The two destinations are mutually exclusive and a piped or remote input is
// rejected while parsing; what is left needs the run mode and the script, so it
// is checked here. sqlScript is the --sql-file contents, or "" for the other
// modes.
func (s *Shell) validateSaveFlags(sqlScript string) error {
	if !s.argument.SaveInPlace && s.argument.SaveTablesDir == "" {
		return nil
	}
	// --sql-file is a non-interactive execution path just like --sql and piped
	// input, so it may carry write-back even when stdin is a TTY. Reject the save
	// flags only for a genuinely interactive run with no query source.,
	if s.argument.Query == "" && s.argument.SQLFilePath == "" && s.isTTY() {
		return errors.New("--save-in-place/--save-tables require --sql, --sql-file, or piped input; use the .save command in the interactive shell")
	}
	// Write-back writes imported tables back to the files they came from, so a run
	// with no input files has nothing to write. A script can create its own tables
	// with .import, so only a run whose statements are all fixed up front — a
	// --sql one-liner, or a --sql-file script with no .import — can be rejected
	// here; the rest is caught by preflightSave before the first statement.
	if len(s.argument.FilePaths) == 0 && !s.scriptMayImport(sqlScript) {
		return errors.New("--save-in-place/--save-tables write imported tables back to their source files, but no input file or directory was given")
	}
	// A write-back run must be able to persist every table it imports: it is
	// all-or-nothing, and a session cannot know up front which tables a statement
	// will touch. planWriteBack enforces that too, but only once the files have
	// been parsed; naming the input here means the run stops before it reads
	// anything.
	if unwritable := unwritableInputs(s.argument.FilePaths); len(unwritable) > 0 {
		return fmt.Errorf("--save-in-place/--save-tables cannot write back %s; write-back supports csv, tsv, ltsv, parquet, ach, and fed files named directly. Drop the flag, or leave that input out of this run",
			strings.Join(unwritable, ", "))
	}
	return nil
}

// unwritableInputs returns the inputs write-back can never persist: a directory
// (whose tables are not a single editable source sqly owns) and a file whose
// format has no writer. A path that cannot be stat-ed is skipped, so the import
// step reports the real path error instead of this check misattributing it.
func unwritableInputs(paths []string) []string {
	var unwritable []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue // let the import report a missing or unreadable path
		}
		if info.IsDir() {
			unwritable = append(unwritable, path+" (a directory import)")
			continue
		}
		if model.FinancialWriteFormat(path) != "" {
			continue // ACH/Fedwire are rebuilt from their whole table set
		}
		if _, _, ok := writableExportTarget(path); !ok {
			unwritable = append(unwritable, path)
		}
	}
	return unwritable
}

// scriptMayImport reports whether the run could still create tables of its own
// after this point: a batch script read from stdin (not yet read here), or a
// --sql-file script containing .import. A plain --sql run cannot.
func (s *Shell) scriptMayImport(sqlScript string) bool {
	if s.argument.Query == "" && s.argument.SQLFilePath == "" {
		return true // batch stdin: the script is read after this check
	}
	return sqlScript != "" && scriptImportsInput(sqlScript)
}

// saveRequested reports whether a non-interactive write-back flag
// (--save-in-place or --save-tables) is set.
func (s *Shell) saveRequested() bool {
	return s.argument != nil && (s.argument.SaveInPlace || s.argument.SaveTablesDir != "")
}

// saveDestDir returns the write-back destination directory: the --save-tables value,
// or "" for --save-in-place.
func (s *Shell) saveDestDir() string {
	if s.argument.SaveTablesDir != "" {
		return s.argument.SaveTablesDir
	}
	return ""
}

// maybeSave runs write-back after a non-interactive run when a save flag is set.
func (s *Shell) maybeSave(ctx context.Context) error {
	switch {
	case s.argument.SaveTablesDir != "":
		return s.writeBack(ctx, s.argument.SaveTablesDir)
	case s.argument.SaveInPlace:
		return s.writeBack(ctx, "")
	default:
		return nil
	}
}

// preflightSave validates write-back before the SQL runs, so a run that cannot
// persist fails before any query output reaches stdout and before any
// file is written. It is a no-op when no save flag is set or
// when the SQL is read-only, because a read-only run skips write-back.
func (s *Shell) preflightSave(ctx context.Context, script string) error {
	if !s.saveRequested() {
		return nil
	}
	// Reject a statement whose effect write-back cannot represent (DDL, schema
	// changes, ANALYZE, maintenance). Only read-only queries and row-modifying DML
	// on imported tables are persisted, so a schema-only run must fail loudly here
	// instead of exiting 0 while leaving the source unchanged.,
	if stmt := firstSaveIncompatibleStatement(script); stmt != "" {
		return fmt.Errorf(
			"--save-in-place/--save-tables cannot persist %q: it changes schema or runs a maintenance statement that has no file write-back; only INSERT/UPDATE/DELETE on imported tables are saved",
			trimGaps(stmt))
	}
	// A script that imports its own input with .import creates its tables while it
	// runs, so they cannot be listed up front. Defer write-back validation to the
	// post-run save, which sees the imported tables.
	if scriptImportsInput(script) {
		return nil
	}
	if !scriptModifiesData(script) {
		return nil
	}
	// Validate every file-backed table up front: at preflight no change has happened
	// yet, so the unchanged-skip is disabled (false) to keep the validation meaningful.
	_, err := s.planWriteBack(ctx, s.saveDestDir(), false)
	return err
}

// finishNonInteractive runs write-back after a non-interactive run, but only when
// a save flag is set and the run actually changed data. A read-only run, an
// EXPLAIN, or a zero-row DML leaves the imported tables unchanged, so write-back
// is skipped to avoid rewriting source files.
func (s *Shell) finishNonInteractive(ctx context.Context) error {
	if s.saveRequested() {
		switch {
		case s.dataChanged:
			// Run write-back first. If it fails, return before flushing the buffered
			// affected counts so stdout stays free of success text.
			if err := s.maybeSave(ctx); err != nil {
				return err
			}
		default:
			// Say so instead of exiting 0 in silence. A run that asked to persist and
			// wrote no file otherwise looks like it succeeded, and the missing
			// destination is only noticed later. The .save command already reports the
			// same thing for the same reason.
			fmt.Fprintln(config.Stderr, noDataChangedMessage)
		}
	}
	// The run succeeded (write-back ran, or there was nothing to write back), so
	// flush the buffered affected counts to stdout now.
	for _, msg := range s.pendingAffected {
		fmt.Fprint(config.Stdout, msg)
	}
	s.pendingAffected = nil
	return nil
}

// writeTarget is a resolved write-back destination. For a tabular source it
// maps one table to one file (format/comp set, setKind ""). For a native
// financial source (ACH/Fedwire) it represents the whole table set reconstructed
// into a single .ach/.fed file: setKind names the format, baseName is the filesql
// registry key, and members lists every table in the set so their baselines can
// be advanced after the write.
type writeTarget struct {
	table    string
	dest     string
	format   model.ExportFormat
	comp     model.Compression
	setKind  string
	baseName string
	members  []string
}

// writeBack persists the current tables to files. When destDir is empty the
// tables are written back over their source files in place (destructive); the
// caller must have asked for --save-in-place. When destDir is set the tables are written
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
	// skipUnchanged: an actual save persists only tables whose content differs from
	// the import baseline, so a session that touched only a TEMP or scratch table,
	// or made net-zero edits, writes nothing instead of rewriting an untouched
	// source. Preflight validation uses the unfiltered plan (see preflightSave).
	targets, err := s.planWriteBack(ctx, destDir, true)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Fprintln(config.Stderr, "no imported table changed in this session; nothing to save")
		return nil
	}
	return s.executeWriteBack(ctx, destDir, targets)
}

// planWriteBack validates that every current table can be written and returns the
// resolved write targets. It reports all problems at once and writes nothing, so
// a session is never partially saved (). For --save-tables it also rejects a
// destination that resolves to the source file () and a destination that
// already exists ().
// skipUnchanged selects whether a table whose content matches its import baseline
// is dropped from the plan. An actual save passes true (persist only real changes);
// preflight passes false (validate every file-backed table up front, before any
// change has happened).
func (s *Shell) planWriteBack(ctx context.Context, destDir string, skipUnchanged bool) ([]writeTarget, error) {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		return nil, noTablesToSaveError(s.isTTY())
	}

	// Count how many tables map to each source so multi-table sources (Excel,
	// ACH/Fedwire) can be rejected, and index the table names for set validation.
	tablesPerSource := make(map[string]int)
	currentTables := make(map[string]bool, len(tables))
	for _, t := range tables {
		currentTables[t.Name()] = true
		if src, ok := s.tableSources[t.Name()]; ok {
			tablesPerSource[src]++
		}
	}

	var targets []writeTarget
	var problems []string
	// Detect destination collisions so two tables never silently overwrite the
	// same output file (defensive: same-name files already collapse to one table
	// at import, but a future rename could break that assumption).
	plannedDest := make(map[string]string)

	// First pass: native financial sources (ACH/Fedwire) are reconstructed from a
	// complete table set into one file, so they are planned per source rather than
	// per table. financialSetSources marks the sources handled here so the
	// per-table pass below skips their member tables.
	financialSetSources := make(map[string]bool)
	for _, t := range tables {
		source, ok := s.tableSources[t.Name()]
		if !ok || financialSetSources[source] {
			continue
		}
		// A directory-imported table is not a single editable source the session
		// owns, even when it happens to be ACH/Fedwire. Leave it for the per-table
		// pass, which rejects directory imports with a clear error, instead of
		// reconstructing a whole-set file the user did not point sqly at directly.
		if s.dirImported[t.Name()] {
			continue
		}
		format := model.FinancialWriteFormat(source)
		if format == "" {
			continue
		}
		financialSetSources[source] = true
		tgt, problem, skip := s.planFinancialSet(ctx, source, format, currentTables, destDir, plannedDest, skipUnchanged)
		switch {
		case problem != "":
			problems = append(problems, problem)
		case skip:
			// No member table changed; nothing to persist for this set.
		default:
			plannedDest[tgt.dest] = tgt.baseName
			targets = append(targets, tgt)
		}
	}

	for _, t := range tables {
		name := t.Name()
		source, ok := s.tableSources[name]
		if ok && financialSetSources[source] {
			// Handled as a whole-set financial target above.
			continue
		}
		if !ok {
			// A SQL-created scratch table has no source file, so it cannot be
			// persisted. It is transient session state, not a dataset the user asked
			// to save, so skip it instead of failing the whole save.
			continue
		}
		// An actual save persists only tables whose content changed. This is checked
		// before the writability and stdin/directory rejections below, so an unchanged
		// JSONL or Excel import is silently skipped rather than reported as unwritable.
		if skipUnchanged && !s.tableChanged(ctx, name) {
			continue
		}
		if source == stdinTableSource {
			problems = append(problems, name+": came from --stdin-format and has no source file to write back to")
			continue
		}
		if isRemoteURL(source) {
			problems = append(problems, fmt.Sprintf("%s: came from a remote URL (%s)", name, source))
			continue
		}
		// A directory import is not a single editable source the session owns, so
		// reject it even though its source may point at a per-file path for
		// --inspect provenance.
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
			// A --save-tables destination that resolves to the source file would
			// overwrite it, defeating the "originals
			// untouched" contract.
			if sameFilePath(dest, source) {
				problems = append(problems, fmt.Sprintf("%s: --save-tables destination %s is the source file; use --save-in-place to overwrite it", name, dest))
				continue
			}
			// Refuse to overwrite a pre-existing destination so --save-tables never
			// silently clobbers an unrelated file. An in-place
			// --save-in-place intentionally overwrites its own source, so this check is
			// scoped to --save-tables.
			if info, statErr := os.Stat(dest); statErr == nil {
				if info.IsDir() {
					problems = append(problems, fmt.Sprintf("%s: destination %s is an existing directory", name, dest))
				} else {
					problems = append(problems, fmt.Sprintf("%s: destination %s already exists; remove it or choose another --save-tables", name, dest))
				}
				continue
			}
		}
		if prev, ok := plannedDest[dest]; ok {
			problems = append(problems, fmt.Sprintf("%s: destination %s collides with table %s", name, dest, prev))
			continue
		}
		plannedDest[dest] = name
		targets = append(targets, writeTarget{table: name, dest: dest, format: format, comp: comp})
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("cannot save session:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return targets, nil
}

// planFinancialSet validates and resolves the write-back target for one native
// financial source (ACH or Fedwire). It returns the target, or a problem string
// describing why the set cannot be saved, or skip=true when no member table
// changed and skipUnchanged is set. The required companion tables must all be
// present, so a set left incomplete by a DROP is rejected with an explicit error
// before any file is written, rather than producing a malformed .ach/.fed.
func (s *Shell) planFinancialSet(ctx context.Context, source, format string, currentTables map[string]bool, destDir string, plannedDest map[string]string, skipUnchanged bool) (writeTarget, string, bool) {
	base := s.usecases.importer.GetTableNameFromFilePath(source)
	label := filepath.Base(source)

	// Member tables: every currently present table imported from this source. Used
	// to detect changes and to advance baselines after a successful write.
	var members []string
	for name, src := range s.tableSources {
		if src == source && currentTables[name] {
			members = append(members, name)
		}
	}

	var required []string
	switch format {
	case model.FinancialFormatACH:
		required = []string{base + "_file_header", base + "_batches", base + "_entries"}
	case model.FinancialFormatFedWire:
		required = []string{base + "_message"}
	}
	var missing []string
	for _, r := range required {
		if !currentTables[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return writeTarget{}, fmt.Sprintf("%s: incomplete %s set; missing required table(s) %s",
			label, strings.ToUpper(format), strings.Join(missing, ", ")), false
	}

	if skipUnchanged {
		changed := false
		for _, m := range members {
			if s.tableChanged(ctx, m) {
				changed = true
				break
			}
		}
		if !changed {
			return writeTarget{}, "", true
		}
	}

	dest := source
	if destDir != "" {
		dest = filepath.Join(destDir, label)
		if sameFilePath(dest, source) {
			return writeTarget{}, fmt.Sprintf("%s: --save-tables destination %s is the source file; use --save-in-place to overwrite it", label, dest), false
		}
		if info, statErr := os.Stat(dest); statErr == nil {
			if info.IsDir() {
				return writeTarget{}, fmt.Sprintf("%s: destination %s is an existing directory", label, dest), false
			}
			return writeTarget{}, fmt.Sprintf("%s: destination %s already exists; remove it or choose another --save-tables", label, dest), false
		}
	}
	if prev, ok := plannedDest[dest]; ok {
		return writeTarget{}, fmt.Sprintf("%s: destination %s collides with %s", label, dest, prev), false
	}
	return writeTarget{table: base, dest: dest, setKind: format, baseName: base, members: members}, "", false
}

// stagedWrite is one target written to a scratch path, waiting to be moved onto
// its destination once every other target has been written too.
type stagedWrite struct {
	target  writeTarget
	staging string
	// baselines names the tables whose import baseline advances once the move
	// lands: the one table for a tabular target, every member for a financial set.
	baselines []string
	// message is the confirmation printed after the move, so a target that never
	// reaches its destination is never announced as saved.
	message string
	// backup is a copy of the destination taken before the commit phase, used to
	// put it back if a later target fails to commit. Empty when the destination
	// did not exist.
	backup string
}

// executeWriteBack writes the planned targets to disk. Callers run planWriteBack
// first, so by this point every target has been validated.
//
// A save covering several files is all-or-nothing. planWriteBack rejects what it
// can see up front, but the ACH and Fedwire writers validate as they encode, so a
// value the format cannot hold is only rejected once that file is being written.
// Writing each target straight to its destination therefore left the earlier ones
// saved and the rest not, with no record of which. Every target is written to a
// scratch path beside its destination first; only when all of them have been
// written are they moved into place.
func (s *Shell) executeWriteBack(ctx context.Context, destDir string, targets []writeTarget) error {
	if destDir != "" {
		if err := os.MkdirAll(destDir, 0o750); err != nil {
			return fmt.Errorf("failed to create save directory %q: %w", destDir, err)
		}
	}

	staged := make([]stagedWrite, 0, len(targets))
	// Discard whatever has been staged unless the moves below claimed it, and
	// discard the backups the commit phase took, so a save leaves behind only what
	// it meant to write.
	defer func() {
		for _, w := range staged {
			_ = os.Remove(w.staging)
			if w.backup != "" {
				_ = os.Remove(w.backup)
			}
		}
	}()

	for _, tgt := range targets {
		w, err := s.stageWriteTarget(ctx, tgt)
		if err != nil {
			return err
		}
		staged = append(staged, w)
	}

	// Copy every destination that already exists before touching any of them, so
	// a commit that fails halfway can put back the ones it already replaced. A
	// commit is a rename where the platform allows one, but not always (see
	// commitStagedFile), and even a rename can fail on the last target after the
	// first has landed.
	for i := range staged {
		backup, err := backupExisting(staged[i].target.dest)
		if err != nil {
			return fmt.Errorf("failed to prepare %s for saving: %w", staged[i].target.dest, err)
		}
		staged[i].backup = backup
	}

	for i, w := range staged {
		if err := commitStagedFile(w.staging, w.target.dest); err != nil {
			rollbackCommitted(staged[:i])
			return fmt.Errorf("failed to move the saved data onto %s: %w", w.target.dest, err)
		}
	}

	for _, w := range staged {
		for _, name := range w.baselines {
			// The file now matches the table, so move the baseline forward. A later
			// .save in the same session then treats the table as unchanged and does not
			// rewrite an identical file.
			s.snapshotBaseline(ctx, name)
		}
		// Write-back is a file-output operation; its confirmation is control-plane
		// output and goes to stderr so stdout stays free of non-data noise.
		fmt.Fprintln(config.Stderr, w.message)
	}
	return nil
}

// rollbackCommitted undoes the commits that already landed, so a save that fails
// partway leaves every destination as it was. It is best effort: a restore that
// fails cannot be reported without hiding the error that caused the rollback,
// which is the one worth showing.
func rollbackCommitted(done []stagedWrite) {
	for i := len(done) - 1; i >= 0; i-- {
		w := done[i]
		if w.backup == "" {
			// The destination did not exist before this save created it.
			_ = os.Remove(w.target.dest)
			continue
		}
		_ = copyOnto(w.backup, w.target.dest)
	}
}

// backupExisting copies path to a temporary file beside it, or returns "" when
// path does not exist yet.
func backupExisting(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return copyToBackup(path)
}

// commitStagedFile moves a staged file onto its destination.
//
// A plain rename is the goal: it is atomic, so nothing ever sees a half-written
// file. Windows refuses to rename over a destination another handle still has
// open, and an in-place save overwrites exactly the files the session imported
// from, so there the staged bytes are copied over it instead. The caller holds a
// copy of the destination and restores it if this fails.
func commitStagedFile(staging, dest string) error {
	// A rename carries the staging file's own mode onto the destination, and the
	// staging file was created 0600. Left alone, saving a world-readable CSV in
	// place would quietly make it owner-only. Take the destination's mode first
	// and put it on the staging file, so the rename preserves it.
	if err := adoptDestinationMode(staging, dest); err != nil {
		return err
	}

	err := os.Rename(staging, dest)
	if err == nil {
		return nil
	}
	if _, statErr := os.Stat(dest); statErr != nil {
		// Nothing was in the way, so the copy cannot help either.
		return err
	}
	return copyOnto(staging, dest)
}

// adoptDestinationMode gives the staged file the permissions of the file it is
// about to replace. A destination that does not exist yet (a --save-tables
// export) keeps the staging file's own mode, which is the conservative 0600 the
// temp file was created with. A chmod failure is reported rather than ignored:
// the caller treats it as a failed commit and restores what was there, which is
// better than landing a file whose permissions are not the ones it had.
func adoptDestinationMode(staging, dest string) error {
	info, err := os.Stat(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.Chmod(staging, info.Mode().Perm())
}

// copyToBackup copies path to a temporary file beside it and returns that path.
func copyToBackup(path string) (string, error) {
	backup, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".sqly-bak*")
	if err != nil {
		return "", err
	}
	name := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	if err := copyOnto(path, name); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

// copyOnto replaces dest's contents with src's, truncating whatever was there.
func copyOnto(src, dest string) error {
	in, err := os.Open(src) //nolint:gosec // src is a file sqly created or was given as the output
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // dest is the destination the caller asked to save to
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// stageWriteTarget writes one target to a scratch path next to its destination
// and returns what the caller needs to finish the save. The scratch file lives in
// the destination's own directory so the later move stays within one filesystem,
// where a rename is atomic; a dot prefix keeps it out of the way of a directory
// listing if the process dies between the write and the move.
func (s *Shell) stageWriteTarget(ctx context.Context, tgt writeTarget) (stagedWrite, error) {
	dir := filepath.Dir(tgt.dest)
	f, err := os.CreateTemp(dir, "."+filepath.Base(tgt.dest)+".sqly-save-*")
	if err != nil {
		return stagedWrite{}, fmt.Errorf("failed to stage the save for %s: %w", tgt.dest, err)
	}
	staging := f.Name()
	// The writers below open the path themselves; the handle is only how the name
	// was reserved.
	if err := f.Close(); err != nil {
		_ = os.Remove(staging)
		return stagedWrite{}, fmt.Errorf("failed to stage the save for %s: %w", tgt.dest, err)
	}

	w := stagedWrite{target: tgt, staging: staging}
	if tgt.setKind != "" {
		if err := s.writeFinancialSet(ctx, tgt, staging); err != nil {
			_ = os.Remove(staging)
			return stagedWrite{}, err
		}
		w.baselines = tgt.members
		w.message = fmt.Sprintf("Saved %s set %s to %s", strings.ToUpper(tgt.setKind), tgt.baseName, tgt.dest)
		return w, nil
	}

	table, err := s.usecases.metadata.List(ctx, tgt.table)
	if err != nil {
		_ = os.Remove(staging)
		return stagedWrite{}, fmt.Errorf("failed to read table %s: %w", tgt.table, err)
	}
	if err := s.usecases.export.DumpTable(staging, table, tgt.format, tgt.comp); err != nil {
		_ = os.Remove(staging)
		return stagedWrite{}, fmt.Errorf("failed to save table %s to %s: %w", tgt.table, tgt.dest, err)
	}
	w.baselines = []string{tgt.table}
	w.message = fmt.Sprintf("Saved %s to %s", tgt.table, tgt.dest)
	return w, nil
}

// writeFinancialSet reconstructs one ACH/Fedwire file from its table set into
// path. The error names the target's real destination, not the scratch path the
// data is written to, so a failure reads the way the user asked for the save.
func (s *Shell) writeFinancialSet(ctx context.Context, tgt writeTarget, path string) error {
	var err error
	switch tgt.setKind {
	case model.FinancialFormatACH:
		err = s.usecases.persistence.DumpACHFile(ctx, tgt.baseName, path)
	case model.FinancialFormatFedWire:
		err = s.usecases.persistence.DumpFedWireFile(ctx, tgt.baseName, path)
	default:
		return fmt.Errorf("unknown financial set kind %q", tgt.setKind)
	}
	if err != nil {
		return fmt.Errorf("failed to save %s set %s to %s: %w", strings.ToUpper(tgt.setKind), tgt.baseName, tgt.dest, err)
	}
	return nil
}

// sameFilePath reports whether two paths resolve to the same file location. It
// resolves symlinks and, when both paths exist, compares file identity, so a
// symlink (or hardlink) alias to an imported source is recognized as the source
// file and cannot bypass the overwrite guard. A non-existent
// path falls back to its cleaned absolute form.
func sameFilePath(a, b string) bool {
	if resolveFilePath(a) == resolveFilePath(b) {
		return true
	}
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB)
	}
	return false
}

// resolveFilePath returns an absolute path with symlinks resolved when the path
// exists, falling back to the cleaned absolute path otherwise.
func resolveFilePath(p string) string {
	abs := p
	if a, err := filepath.Abs(p); err == nil {
		abs = a
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
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
	// bzip2 has no writer, so a .bz2 source cannot be written back. Reject it here
	// during preflight, before any destination file is created or truncated, so a
	// failed write-back never leaves an empty or corrupted file behind.
	if comp == model.CompressionBzip2 {
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
