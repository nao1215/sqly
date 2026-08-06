package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// inPlaceArg is the .save argument that selects destructive in-place overwrite.
const inPlaceArg = "--in-place"

// followSymlinksArg opts an in-place save into writing through a symlinked
// source. See planSymlinkPolicy for why that needs asking for.
const followSymlinksArg = "--follow-symlinks"

// noDataChangedMessage explains a save that wrote nothing because the session
// left every table as imported, so a read-only session never looks like a
// successful write-back that silently produced no file.
const noDataChangedMessage = "no table data changed in this session; nothing to save"

// saveCommand writes the current tables back to files from the interactive
// shell. ".save DIR" writes into a directory without touching the sources;
// ".save --in-place" overwrites the source files.
func (c CommandList) saveCommand(ctx context.Context, s *Shell, argv []string) error {
	// --follow-symlinks modifies an in-place save and nothing else, so it is
	// stripped here and validated against the destination below. Taking it as a
	// second positional argument would report it as "too many arguments", which
	// says nothing about why it was refused.
	followSymlinks := false
	if i := slices.Index(argv, followSymlinksArg); i >= 0 {
		followSymlinks = true
		argv = append(append([]string{}, argv[:i]...), argv[i+1:]...)
	}

	if len(argv) != 1 {
		// A missing or extra argument is a command error so a batch script fails
		// fast instead of skipping the save and exiting 0. The usage and note ride
		// on the error path.
		return &invocationError{Err: errors.New(".save requires a single argument: a directory or --in-place\n" +
			"[Usage]\n" +
			"  .save DIRECTORY   write each table into DIRECTORY (originals untouched)\n" +
			"  .save --in-place  overwrite each table's source file\n" +
			"                    add --follow-symlinks to write through a symlinked source\n" +
			"[Note]\n" +
			"  csv/tsv/ltsv/parquet sources are written; compression is preserved.\n" +
			"  A whole ACH/Fedwire set is reconstructed back into a single .ach/.fed file\n" +
			"  when all of that source's tables are still present")}
	}
	// The option only means anything for the destructive form. `.save DIR
	// --follow-symlinks` reads as though it changes something about the export,
	// and there is nothing there for it to change.
	if followSymlinks && argv[0] != inPlaceArg {
		return &invocationError{Err: fmt.Errorf(".save %s applies to %s only; .save DIR writes elsewhere and never follows a source link",
			followSymlinksArg, inPlaceArg)}
	}
	// Reject an empty destination so `.save ""` is not silently treated as an
	// in-place save, which the user never asked for.
	if argv[0] != inPlaceArg && strings.TrimSpace(argv[0]) == "" {
		return &invocationError{Err: errors.New(".save requires a non-empty directory; use .save --in-place to overwrite the sources")}
	}
	// Anything else beginning with "-" is a flag the user meant, not a directory
	// they want created. Taking it as a destination would silently write a
	// directory named after the flag instead of doing what was asked.
	if argv[0] != inPlaceArg && strings.HasPrefix(argv[0], "-") {
		return &invocationError{Err: fmt.Errorf(".save does not have a %s option; write to a directory with .save DIR, or overwrite the sources with .save %s", argv[0], inPlaceArg)}
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
		return s.writeBack(ctx, "", followSymlinks)
	}
	// Expand a leading "~" so `.save ~/out` writes under the home directory
	// instead of a literal "~" directory.
	destDir, err := expandTilde(argv[0])
	if err != nil {
		return err
	}
	return s.writeBack(ctx, destDir, followSymlinks)
}

// noTablesToSaveError builds the empty-session save error with recovery guidance
// tailored to the run mode. Save is safety-sensitive, so the message names the
// next step (.import a file, or pass input files) instead of a bare "no tables to
// save".
func noTablesToSaveError(interactive bool) error {
	if interactive {
		return &writeBackError{Err: errors.New("no tables to save: run .import FILE to load a table first")}
	}
	return &writeBackError{Err: errors.New("no tables to save: pass input files (e.g. sqly data.csv ...) before saving")}
}

// preflightSave rejects a script whose statements .save could never persist,
// before the first of them runs. It checks the statements alone; which tables
// can be written is left to .save, which sees the tables the session actually
// changed and reports them the same way whether it was typed at the prompt or
// read from a script.
func (s *Shell) preflightSave(elements []scriptElement) error {
	if !runsHelper(elements, saveCommand) {
		return nil
	}
	// Reject a statement whose effect write-back cannot represent (DDL, schema
	// changes, ANALYZE, maintenance). Only read-only queries and row-modifying DML
	// on imported tables are persisted, so a schema-only run must fail loudly here
	// instead of exiting 0 while leaving the source unchanged.,
	if stmt := firstSaveIncompatibleStatement(elements); stmt != "" {
		return fmt.Errorf(
			".save cannot persist %q: it changes schema or runs a maintenance statement that has no file write-back; only INSERT/UPDATE/DELETE on imported tables are saved",
			trimGaps(stmt))
	}
	return nil
}

// finishNonInteractive flushes the affected-row counts a non-interactive run
// buffered. They are buffered rather than printed as they happen so that a run
// which fails after a DML statement leaves stdout free of success text.
func (s *Shell) finishNonInteractive(_ context.Context) error {
	s.flushPendingAffected()
	return nil
}

// flushPendingAffected prints the buffered counts and empties the buffer. A
// successful write-back calls it before reporting the files it wrote, so the
// counts read in statement order; a run that ends without one calls it at the
// end. Calling it twice is harmless, which is what makes both paths safe.
func (s *Shell) flushPendingAffected() {
	for _, msg := range s.pendingAffected {
		fmt.Fprint(config.Stdout, msg)
	}
	s.pendingAffected = nil
}

// destinationIndex records which table has claimed each destination path, so a
// second table cannot be planned onto a file the first will write.
//
// Paths are compared case-folded. A case-sensitive filesystem would allow
// "Sales.csv" and "sales.csv" side by side, but macOS and Windows would not, and
// a save that succeeds on Linux and silently overwrites one table with another
// on a laptop is worse than a save that refuses everywhere.
type destinationIndex map[string]string

func newDestinationIndex() destinationIndex { return make(destinationIndex) }

// claim records dest as belonging to table.
func (d destinationIndex) claim(dest, table string) { d[strings.ToLower(dest)] = table }

// claimedBy returns the table that already claimed dest, if any.
func (d destinationIndex) claimedBy(dest string) (string, bool) {
	table, ok := d[strings.ToLower(dest)]
	return table, ok
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
// caller must have asked for .save --in-place. When destDir is set the tables are written
// into that directory, preserving each source's file name, and the original
// source files are left untouched.
//
// A table backed by its own file — CSV, TSV, LTSV, Parquet — is written back in
// that format, with the source's compression preserved. The several tables of an
// ACH or Fedwire source are reconstructed together into the one file they came
// from. Anything that cannot be written that way is rejected before the first
// byte is written, so a session is never partially saved.
func (s *Shell) writeBack(ctx context.Context, destDir string, followSymlinks bool) (err error) {
	// Whatever stopped a save, it stopped a write, and the exit code says so.
	// The classification is applied here rather than at each failure because
	// every one of them is the same class: a save that could not create its
	// directory, could not stage a file, or could not move one into place is a
	// destination that could not be written, which is a 4. Most of them were
	// plain fmt.Errorf and fell through to the generic 1, so a wrapper could not
	// tell "your SQL was wrong" from "the disk is read-only".
	defer func() {
		if err == nil {
			return
		}
		var already *writeBackError
		if !errors.As(err, &already) {
			err = &writeBackError{Err: err}
		}
	}()

	// Both destinations skip a table the session did not change, and they mean
	// different things by "did not change" — see planWriteBack.
	targets, err := s.planWriteBack(ctx, destDir, true)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Fprintln(config.Stderr, "no imported table changed in this session; nothing to save")
		return nil
	}
	if err := s.applySymlinkPolicy(destDir, targets, followSymlinks); err != nil {
		return err
	}
	return s.executeWriteBack(ctx, destDir, targets)
}

// applySymlinkPolicy decides whether an in-place save may write through a
// symlinked source, and says where the write is going when it may.
//
// An in-place save overwrites the file it read, and a symlink makes "the file it
// read" two answers: the link the user typed and the file behind it. sqly writes
// to the second — every step of the write resolves the link, which is what keeps
// a save from replacing the link with a regular file and leaving the real one
// holding the old rows. That is the right way to follow a link and it is still
// worth asking about, because the path being overwritten is one the user never
// named. It can sit outside the directory they are working in, and it can be
// shared with something that did not expect sqly to rewrite it.
//
// So the default refuses and names the link, and --follow-symlinks is how a user
// who meant it says so. When they do, the resolved path goes to stderr: the
// whole reason for asking is that the destination is not what was typed, so
// proceeding silently would answer the question by ignoring it.
//
// This is scoped to the in-place form. `.save DIR` writes somewhere else and
// leaves every source alone, so a symlinked source is not a hazard there.
func (s *Shell) applySymlinkPolicy(destDir string, targets []writeTarget, followSymlinks bool) error {
	if destDir != "" {
		return nil
	}

	var problems []string
	for _, tgt := range targets {
		info, err := os.Lstat(tgt.dest)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			// A path that cannot be stat'd is left to the write itself, which
			// reports filesystem failures with the phase they happened in.
			continue
		}
		resolved, err := filepath.EvalSymlinks(tgt.dest)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s is a symlink that does not resolve: %v",
				tgt.table, tgt.dest, err))
			continue
		}
		if !followSymlinks {
			problems = append(problems, fmt.Sprintf(
				"%s: %s is a symlink to %s; an in-place save would overwrite that file, which you did not name. Add %s to do it anyway, or save to a directory with .save DIR",
				tgt.table, tgt.dest, resolved, followSymlinksArg))
			continue
		}
		fmt.Fprintf(config.Stderr, "following the symlink %s to %s\n", tgt.dest, resolved)
	}

	if len(problems) > 0 {
		return &writeBackError{Err: fmt.Errorf("cannot save session:\n  - %s", strings.Join(problems, "\n  - "))}
	}
	return nil
}

// planWriteBack validates that every current table can be written and returns the
// resolved write targets. It reports all problems at once and writes nothing, so
// a session is never partially saved. For .save DIR it also rejects a
// destination that resolves to the source file, and one that already exists.
//
// skipUnchanged selects whether an unchanged table is dropped from the plan. An
// actual save passes true (persist only real changes); preflight passes false
// (validate every file-backed table up front, before any change has happened).
// What counts as unchanged depends on the destination — see tableNeedsWriting.
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
	// same output file. Same-name inputs already collapse to one table at import,
	// but a destination is a path, not a table name: two tables whose names differ
	// only in case land on one file on macOS and Windows, where the filesystem
	// does not distinguish them. The key is case-folded so that collision is
	// caught on every platform rather than only where it happens to bite.
	plannedDest := newDestinationIndex()

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
			plannedDest.claim(tgt.dest, tgt.baseName)
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
		if skipUnchanged && !s.tableNeedsWriting(ctx, name, destDir) {
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
			// An ACH or Fedwire source never reaches here: its tables are written as
			// one set above. What is left is a workbook, where the sheets are
			// separate tables of one file and sqly has no Excel writer to rebuild it.
			problems = append(problems, fmt.Sprintf("%s: shares source %s with the other sheets of that workbook", name, source))
			continue
		}
		format, comp, reason := exportTargetFor(source)
		if reason != nil {
			problems = append(problems, fmt.Sprintf("%s: cannot write back to %s: %v", name, filepath.Base(source), reason))
			continue
		}

		dest := source
		if destDir != "" {
			dest = filepath.Join(destDir, filepath.Base(source))
			// A .save DIR destination that resolves to the source file would
			// overwrite it, defeating the "originals
			// untouched" contract.
			if sameFilePath(dest, source) {
				problems = append(problems, fmt.Sprintf("%s: destination %s is the source file; use .save --in-place to overwrite it", name, dest))
				continue
			}
			// Refuse to overwrite a pre-existing destination so .save DIR never
			// silently clobbers an unrelated file. An in-place
			// .save --in-place intentionally overwrites its own source, so this check is
			// scoped to .save DIR.
			if info, statErr := os.Stat(dest); statErr == nil {
				if info.IsDir() {
					problems = append(problems, fmt.Sprintf("%s: destination %s is an existing directory", name, dest))
				} else {
					problems = append(problems, fmt.Sprintf("%s: destination %s already exists; remove it or choose another directory", name, dest))
				}
				continue
			}
		}
		if prev, ok := plannedDest.claimedBy(dest); ok {
			problems = append(problems, fmt.Sprintf("%s: destination %s collides with the one already planned for table %s", name, dest, prev))
			continue
		}
		plannedDest.claim(dest, name)
		targets = append(targets, writeTarget{table: name, dest: dest, format: format, comp: comp})
	}

	if len(problems) > 0 {
		return nil, &writeBackError{Err: fmt.Errorf("cannot save session:\n  - %s", strings.Join(problems, "\n  - "))}
	}
	return targets, nil
}

// tableNeedsWriting reports whether a save to this destination has anything to
// write for a table.
//
// The two destinations ask different questions, because they write different
// files. `.save --in-place` rewrites the source, so it asks whether the table
// still differs from what the source holds: a table already written out needs no
// second write. `.save DIR` writes somewhere else — usually a file that does not
// exist yet — so the source is irrelevant, and the question is whether the
// session changed the table at all.
//
// Sharing one answer broke both directions in turn. With the export moving the
// source's baseline, `UPDATE; .save out; .save --in-place` left the source with
// its old rows. With the export reading the source's baseline,
// `UPDATE; .save --in-place; .save out` wrote no export at all. They are two
// questions and now there are two baselines.
func (s *Shell) tableNeedsWriting(ctx context.Context, name, destDir string) bool {
	if destDir == "" {
		return s.tableNeedsSourceWrite(ctx, name)
	}
	return s.tableChanged(ctx, name)
}

// planFinancialSet validates and resolves the write-back target for one native
// financial source (ACH or Fedwire). It returns the target, or a problem string
// describing why the set cannot be saved, or skip=true when no member table
// changed and skipUnchanged is set. The required companion tables must all be
// present, so a set left incomplete by a DROP is rejected with an explicit error
// before any file is written, rather than producing a malformed .ach/.fed.
func (s *Shell) planFinancialSet(ctx context.Context, source, format string, currentTables map[string]bool, destDir string, plannedDest destinationIndex, skipUnchanged bool) (writeTarget, string, bool) {
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
			// A financial set is one file, so it asks the same question its
			// destination asks of every other table.
			if s.tableNeedsWriting(ctx, m, destDir) {
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
			return writeTarget{}, fmt.Sprintf("%s: destination %s is the source file; use .save --in-place to overwrite it", label, dest), false
		}
		if info, statErr := os.Stat(dest); statErr == nil {
			if info.IsDir() {
				return writeTarget{}, fmt.Sprintf("%s: destination %s is an existing directory", label, dest), false
			}
			return writeTarget{}, fmt.Sprintf("%s: destination %s already exists; remove it or choose another directory", label, dest), false
		}
	}
	if prev, ok := plannedDest.claimedBy(dest); ok {
		return writeTarget{}, fmt.Sprintf("%s: destination %s collides with the one already planned for %s", label, dest, prev), false
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
			_ = s.fs().Remove(w.staging)
			if w.backup != "" {
				_ = s.fs().Remove(w.backup)
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
		backup, err := s.backupExisting(staged[i].target.dest)
		if err != nil {
			return fmt.Errorf("failed to prepare %s for saving: %w", staged[i].target.dest, err)
		}
		staged[i].backup = backup
	}

	for i, w := range staged {
		// Every destination was copied aside before this loop began, so the
		// fallback needs no further preparation here.
		touched, err := s.commitStagedFile(w.staging, w.target.dest, nil)
		if err == nil {
			continue
		}
		commitErr := fmt.Errorf("failed to move the saved data onto %s: %w", w.target.dest, err)
		// The target that just failed is rolled back with the ones before it when
		// it was touched. It is the likeliest of all of them to be broken: a
		// fallback copy truncates before it writes, so the file it failed on is
		// the one holding half a table. Leaving it out of the rollback — which is
		// what "undo the commits that already landed" reads like — restores every
		// file except the damaged one.
		done := staged[:i]
		if touched {
			done = staged[:i+1]
		}
		// The rollback's own failure is reported alongside the commit failure,
		// never instead of it. Both matter and they say different things: the
		// commit error is why the save stopped, and the rollback error is which
		// files are now neither the old version nor the new one. Dropping the
		// second leaves the user believing the sources are untouched.
		if rollbackErr := s.rollbackCommitted(done); rollbackErr != nil {
			return errors.Join(commitErr, rollbackErr)
		}
		return commitErr
	}

	// Every destination is committed, so the run has succeeded and the counts the
	// statements produced can be released. They go out before the "Saved" lines
	// because that is the order the things happened in: the rows changed, then
	// the files were written.
	s.flushPendingAffected()

	// The baseline is what the *source* file holds, and it is the answer to "has
	// this table changed since it was imported?". Only an in-place save makes the
	// source match the table, so only an in-place save may move it forward.
	//
	// A `.save DIR` export used to advance it too, which said the source had
	// caught up when the export had. `UPDATE; .save out; .save --in-place` then
	// wrote the export and reported "nothing to save" for the source — the one
	// sequence where the contract that DIR leaves the sources alone turns into
	// the sources never being writable again.
	inPlace := destDir == ""
	for _, w := range staged {
		if inPlace {
			for _, name := range w.baselines {
				s.snapshotSourceFromTable(ctx, name)
			}
		}
		// Write-back is a file-output operation; its confirmation is control-plane
		// output and goes to stderr so stdout stays free of non-data noise.
		fmt.Fprintln(config.Stderr, w.message)
	}
	return nil
}

// rollbackCommitted undoes the commits that already landed, so a save that fails
// partway leaves every destination as it was. It returns what it could not
// restore: a destination left holding new content after the save failed is the
// one thing the user has to know about, and it must not be swallowed just
// because another error is already on its way up. Every destination is attempted
// even after one fails, so the report covers all of them.
func (s *Shell) rollbackCommitted(done []stagedWrite) error {
	var failures []error
	for i := len(done) - 1; i >= 0; i-- {
		w := done[i]
		if err := s.restoreFromBackup(w.target.dest, w.backup); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// backupExisting copies path to a temporary file beside it, or returns "" when
// path does not exist yet.
func (s *Shell) backupExisting(path string) (string, error) {
	if _, err := s.fs().Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return s.copyToBackup(path)
}

// commitStagedFile moves a staged file onto its destination. It reports whether
// the destination was touched, which is the only thing the caller cannot work
// out for itself after a failure.
//
// A plain rename is the goal: it is atomic, so nothing ever sees a half-written
// file, and a rename that fails leaves the destination untouched. Windows
// refuses to rename over a destination another handle still has open, and an
// in-place save overwrites exactly the files the session imported from, so there
// the staged bytes are copied over it instead.
//
// That fallback is not atomic and cannot be made so: it truncates the
// destination and then writes. A disk that fills up on the third block leaves
// the destination truncated or half-written, and no ordering of the copy avoids
// that. So the fallback reports touched=true before it can fail, and every
// caller of this function must hold a backup of the destination and restore it
// when touched is true — which is why the return value exists rather than a
// comment saying "be careful here".
func (s *Shell) commitStagedFile(staging, dest string, beforeFallback func() error) (touched bool, err error) {
	// A rename replaces the name, not the file the name points at. Where the
	// destination is a symlink that means the link itself is replaced by a
	// regular file: the link is gone, the file it pointed at still holds the old
	// rows, and sqly says "Saved". Everything else here already follows the link —
	// Stat, and the copy that opens the destination for writing — so the rename
	// is the one step that has to be told to, by being pointed at the real file.
	//
	// A destination that does not exist resolves to itself, which is what a
	// `.save DIR` export and a new `--output` file want.
	dest = resolveFilePath(dest)

	// A rename carries the staging file's own mode onto the destination, and the
	// staging file was created 0600. Left alone, saving a world-readable CSV in
	// place would quietly make it owner-only. Take the destination's mode first
	// and put it on the staging file, so the rename preserves it.
	if err := s.adoptDestinationMode(staging, dest); err != nil {
		return false, err
	}

	err = s.fs().Rename(staging, dest)
	if err == nil {
		return true, nil
	}
	if _, statErr := s.fs().Stat(dest); statErr != nil {
		// Nothing was in the way, so the copy cannot help either, and nothing was
		// written: a rename that fails does not create its destination.
		return false, err
	}
	// The rename was refused and the destination exists, so the fallback is about
	// to overwrite it. beforeFallback is the caller's last chance to hold a copy;
	// a caller that already has one passes nil.
	if beforeFallback != nil {
		if prepErr := beforeFallback(); prepErr != nil {
			return false, prepErr
		}
	}
	// From here the destination may end up truncated, partly written, or whole.
	// The caller restores it from its backup either way.
	if copyErr := s.fs().Copy(staging, dest); copyErr != nil {
		return true, copyErr
	}
	return true, nil
}

// adoptDestinationMode gives the staged file the permissions of the file it is
// about to replace. A destination that does not exist yet (a .save DIR
// export) keeps the staging file's own mode, which is the conservative 0600 the
// temp file was created with. A chmod failure is reported rather than ignored:
// the caller treats it as a failed commit and restores what was there, which is
// better than landing a file whose permissions are not the ones it had.
func (s *Shell) adoptDestinationMode(staging, dest string) error {
	info, err := s.fs().Stat(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return s.fs().Chmod(staging, info.Mode().Perm())
}

// copyToBackup copies path to a temporary file beside it and returns that path.
func (s *Shell) copyToBackup(path string) (string, error) {
	name, err := s.fs().stagingPath(path, ".sqly-bak*")
	if err != nil {
		return "", err
	}
	if err := s.fs().Copy(path, name); err != nil {
		_ = s.fs().Remove(name)
		return "", err
	}
	return name, nil
}

// stageWriteTarget writes one target to a scratch path next to its destination
// and returns what the caller needs to finish the save. The scratch file lives in
// the destination's own directory so the later move stays within one filesystem,
// where a rename is atomic; a dot prefix keeps it out of the way of a directory
// listing if the process dies between the write and the move.
func (s *Shell) stageWriteTarget(ctx context.Context, tgt writeTarget) (stagedWrite, error) {
	staging, err := s.fs().stagingPath(tgt.dest, ".sqly-save-*")
	if err != nil {
		return stagedWrite{}, fmt.Errorf("failed to stage the save for %s: %w", tgt.dest, err)
	}

	w := stagedWrite{target: tgt, staging: staging}
	if tgt.setKind != "" {
		if err := s.writeFinancialSet(ctx, tgt, staging); err != nil {
			_ = s.fs().Remove(staging)
			return stagedWrite{}, err
		}
		w.baselines = tgt.members
		w.message = fmt.Sprintf("Saved %s set %s to %s", strings.ToUpper(tgt.setKind), tgt.baseName, tgt.dest)
		return w, nil
	}

	table, err := s.usecases.metadata.List(ctx, tgt.table)
	if err != nil {
		_ = s.fs().Remove(staging)
		return stagedWrite{}, fmt.Errorf("failed to read table %s: %w", tgt.table, err)
	}
	if err := s.usecases.export.DumpTable(staging, table, tgt.format, tgt.comp); err != nil {
		_ = s.fs().Remove(staging)
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
	format, comp, err := exportTargetFor(source)
	if err != nil {
		return 0, model.CompressionNone, false
	}
	return format, comp, true
}

// exportTargetFor is writableExportTarget with the reason it said no.
//
// Three different things make a source unwritable, and they need three different
// answers. Reporting them all as "write-back to data.csv.bz2 is not supported
// (use csv, tsv, ltsv, or parquet)" told a user holding a CSV to use CSV, and a
// user holding a Parquet to use Parquet: the format was never the problem in
// either case, the compression was. The reason is named here so the caller can
// say which one it hit.
func exportTargetFor(source string) (model.ExportFormat, model.Compression, error) {
	comp := model.CompressionNone
	base := source
	if c, ok := model.CompressionFromExtension(filepath.Ext(source)); ok {
		comp = c
		base = strings.TrimSuffix(source, filepath.Ext(source))
	}
	format, ok := model.ExportFormatFromExtension(filepath.Ext(base))
	if !ok {
		return 0, model.CompressionNone, errUnwritableFormat
	}
	// bzip2 has no writer, so a .bz2 source cannot be written back. Reject it here
	// during preflight, before any destination file is created or truncated, so a
	// failed write-back never leaves an empty or corrupted file behind.
	if comp == model.CompressionBzip2 {
		return 0, model.CompressionNone, errUnwritableBzip2
	}
	switch format {
	case model.ExportCSV, model.ExportTSV, model.ExportLTSV:
		return format, comp, nil
	case model.ExportParquet:
		if comp != model.CompressionNone {
			return 0, model.CompressionNone, errUnwritableCompressedParquet
		}
		return format, model.CompressionNone, nil
	default:
		return 0, model.CompressionNone, errUnwritableFormat
	}
}

// The reasons a source cannot be written back, each with the next step that
// actually applies to it.
var (
	errUnwritableFormat = errors.New(
		"sqly reads this format but cannot write it back; export the table with .dump instead")
	errUnwritableBzip2 = errors.New(
		"the format is writable but bzip2 has no writer in Go, so the file cannot be rebuilt; export the table with .dump instead")
	errUnwritableCompressedParquet = errors.New(
		"the format is writable but a compressed parquet cannot be rebuilt; export the table with .dump instead")
)
