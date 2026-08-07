package shell

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// An import of several inputs is one operation, not a sequence of them.
//
// It used to be a sequence: each path was loaded in its own transaction, and a
// malformed file in the middle left the ones before it in the database and
// stopped the ones after it from being read at all. The run then exited
// non-zero, so a script saw failure — and a session saw a database holding some
// of what was asked for, with no way to tell which part. "It failed" and "it
// half worked" were the same outcome.
//
// So the work is split into phases, and nothing touches the database until
// every input has been resolved and every table name it will claim is known:
//
//	resolve   — download remote inputs, expand directories, stage what needs
//	            staging. Any failure here ends the import having written nothing.
//	preflight — work out which tables each input will create, and refuse two
//	            inputs that want the same one.
//	load      — one call, one transaction, all of the inputs or none of them.
//	record    — sources, directory markers, baselines, and workbook sheet
//	            records, applied only after the load has committed.
//
// The order the user gave is the order everything runs in, so which input a
// failure names does not change between runs.

// importTarget is one file an import will load, after resolution.
type importTarget struct {
	// loadPath is the path handed to filesql: the staged copy of a download or a
	// re-encoded text file, or the file itself.
	loadPath string
	// displayPath is what the user named — a path, or the URL a download came
	// from. It is what errors quote and what a table's source records.
	displayPath string
	// fromDirectory marks a file found by expanding a directory argument, so
	// write-back can keep refusing it: a directory is not one editable source.
	fromDirectory bool
}

// inputName is what to call an input in a message. A staged --stdin-format
// dataset lives at a random temp path, which is sqly's business and not an
// answer to "which two inputs collided": the reader cannot rename it, and the
// path is gone by the time they read about it. Import failures already scrub it;
// this gives the same name to the checks that run before the import.
func (s *Shell) inputName(target importTarget) string {
	if s.stdinStagedPath != "" && target.loadPath == s.stdinStagedPath {
		return fmt.Sprintf("stdin (--stdin-format %s)", s.argument.StdinFormat)
	}
	return target.displayPath
}

// importPlan is a resolved import: every file it will read, in order, and the
// cleanups owed for the temporary copies resolution made.
type importPlan struct {
	targets  []importTarget
	cleanups []func()
	// seen holds the source of every target already planned, so one file named
	// twice is read once. A user can name a file and the directory holding it in
	// the same command, or repeat a path outright; neither means "load it twice",
	// and without this the collision check would report the file as colliding
	// with itself.
	seen []string
	// directoryLabels names the directory arguments, as the user wrote them, for
	// the banner a successful directory import prints.
	directoryLabels []string
}

// alreadyPlanned reports whether this source is already in the plan.
func (p *importPlan) alreadyPlanned(source string) bool {
	for _, planned := range p.seen {
		if sameSourceLocation(planned, source) {
			return true
		}
	}
	return false
}

// release runs every cleanup the plan accumulated, newest first.
func (p *importPlan) release() {
	for i := len(p.cleanups) - 1; i >= 0; i-- {
		p.cleanups[i]()
	}
	p.cleanups = nil
}

// loadPaths returns the paths to hand filesql, in the planned order.
func (p *importPlan) loadPaths() []string {
	paths := make([]string, 0, len(p.targets))
	for _, target := range p.targets {
		paths = append(paths, target.loadPath)
	}
	return paths
}

// resolveImportPlan turns the paths a user named into the files that will be
// read, downloading and expanding as it goes.
//
// It writes nothing to the database. A failure here — an unreachable URL, a
// missing path, a directory with nothing supported in it — ends the import with
// the session exactly as it was, which is the first half of "all or nothing".
func (s *Shell) resolveImportPlan(ctx context.Context, argv, labels []string) (*importPlan, error) {
	// The remote capability is checked across every input before the first one is
	// resolved, so a mix of local files and a URL this session may not download
	// refuses without staging the local half. It is checked here rather than only
	// at startup because this is the funnel every import passes through: the
	// positional arguments, `.import` at the prompt, and `.import` inside a
	// script all arrive at this function.
	if err := s.authorizeRemoteInputs(argv); err != nil {
		return nil, err
	}

	plan := &importPlan{}
	for i, input := range argv {
		// The label is what the user wrote, which is not always the path being
		// resolved: an internal caller may resolve a temporary directory while the
		// user named something else. Messages quote the label.
		label := input
		if i < len(labels) {
			label = labels[i]
		}
		if strings.TrimSpace(input) == "" {
			plan.release()
			return nil, &invocationError{Err: errors.New(".import was given an empty path\n" + importUsageText())}
		}

		cleanPath, cleanup, info, err := s.resolveImportTarget(ctx, input)
		if cleanup != nil {
			plan.cleanups = append(plan.cleanups, cleanup)
		}
		if err != nil {
			plan.release()
			return nil, err
		}

		if info.IsDir() {
			plan.directoryLabels = append(plan.directoryLabels, label)
			if err := s.planDirectory(plan, cleanPath, label); err != nil {
				plan.release()
				return nil, err
			}
			continue
		}
		if err := s.planFile(plan, cleanPath, label, false); err != nil {
			plan.release()
			return nil, err
		}
	}
	if len(plan.targets) == 0 {
		plan.release()
		return nil, errors.New("no supported files to import")
	}
	return plan, nil
}

// planDirectory adds every supported file under a directory argument.
func (s *Shell) planDirectory(plan *importPlan, cleanPath, displayPath string) error {
	files, err := s.supportedFilesInDir(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to scan directory %s: %w", displayPath, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported files found in directory %s", displayPath)
	}
	// supportedFilesInDir sorts, so a directory contributes its files in the same
	// order on every platform. Without that, which file a failing import names
	// would depend on the filesystem's readdir order.
	for _, file := range files {
		if err := s.planFile(plan, file, file, true); err != nil {
			return fmt.Errorf("failed to prepare %s from directory %s: %w", file, displayPath, err)
		}
	}
	return nil
}

// planFile adds one file, staging it first when the format or the encoding
// needs it.
func (s *Shell) planFile(plan *importPlan, cleanPath, displayPath string, fromDirectory bool) error {
	// Identity is decided on the file itself, before any staging: a staged copy
	// gets a fresh temp path every time and would never match anything.
	if plan.alreadyPlanned(cleanPath) {
		return nil
	}
	plan.seen = append(plan.seen, cleanPath)

	loadPath := cleanPath
	if !s.usecases.importer.IsSupportedFile(cleanPath) {
		staged, cleanup, ok := s.stagePseudoFileAsCSV(cleanPath)
		if !ok {
			return fmt.Errorf("unsupported file format: %s (supported: csv, tsv, ltsv, json, jsonl, parquet, xlsx [+compressed], ach, fed)",
				filepath.Base(cleanPath))
		}
		plan.cleanups = append(plan.cleanups, cleanup)
		loadPath = staged
	}

	prepared, cleanup, err := s.prepareImportLoadPath(loadPath)
	if err != nil {
		return err
	}
	if cleanup != nil {
		plan.cleanups = append(plan.cleanups, cleanup)
	}

	plan.targets = append(plan.targets, importTarget{
		loadPath:      prepared,
		displayPath:   displayPath,
		fromDirectory: fromDirectory,
	})
	return nil
}

// claimedTables is the set of tables one target will create, in a stable order.
type claimedTables struct {
	target importTarget
	tables []string
}

// preflightTableNames works out which tables each input will create and refuses
// two inputs that want the same one.
//
// The check has to happen before the load, not during it. Two files that both
// want the table "book" are not a thing to resolve by picking one — whichever
// were picked, the other file's rows would be missing and nothing would say so.
// Deciding it here means the refusal names both sources and the database is
// untouched.
//
// A table an input will overwrite because it produced that table earlier in this
// session is not a collision: re-importing a file the session already holds is
// an ordinary thing to do.
func (s *Shell) preflightTableNames(ctx context.Context, plan *importPlan) ([]claimedTables, error) {
	existing, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get table names before importing: %w", err)
	}
	existingSet := tableNameSet(existing)

	// SQLite compares table names case-insensitively for ASCII, so two inputs
	// differing only in case would still want one table.
	claimedBy := make(map[string]importTarget, len(plan.targets))
	claims := make([]claimedTables, 0, len(plan.targets))

	for _, target := range plan.targets {
		tables, err := s.tablesClaimedBy(target, existingSet)
		if err != nil {
			return nil, err
		}
		for _, table := range tables {
			key := strings.ToLower(table)
			if previous, taken := claimedBy[key]; taken {
				return nil, fmt.Errorf(
					"table-name collision: %s and %s both map to table %q; rename one of them or import them separately",
					s.inputName(previous), s.inputName(target), table)
			}
			claimedBy[key] = target
		}
		claims = append(claims, claimedTables{target: target, tables: tables})
	}
	return claims, nil
}

// tablesClaimedBy returns the tables an input will create.
//
// A workbook is asked directly, because its sheets decide its tables and the
// sheet policy decides which sheets count — the same answer the load will
// reach. Everything else is named after its file; the formats that produce
// several tables from one file (ACH, Fedwire) claim their base name here and
// are attributed exactly after the load, when the tables exist to be seen.
func (s *Shell) tablesClaimedBy(target importTarget, existing map[string]struct{}) ([]string, error) {
	base := s.usecases.importer.GetTableNameFromFilePath(target.loadPath)

	if s.usecases.importer.IsExcelFile(target.loadPath) {
		sheets, err := s.usecases.importer.ExcelSheets(target.loadPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", target.displayPath, err)
		}
		loaded := make([]string, 0, len(sheets))
		for _, sheet := range sheets {
			if sheet.Visible || s.usecases.importer.IncludeHiddenSheets() {
				loaded = append(loaded, sheet.Name)
			}
		}
		tables, err := s.usecases.importer.ExcelSheetTableNames(target.loadPath, loaded)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", target.displayPath, err)
		}
		return tables, nil
	}

	// A file re-imported over the table it already owns is not claiming a name
	// from anyone, so it is left out of the collision check.
	if _, taken := existing[base]; taken && s.ownsTable(base, target.displayPath) {
		return nil, nil
	}
	return []string{base}, nil
}

// ownsTable reports whether this source is the one a table was recorded against.
func (s *Shell) ownsTable(table, source string) bool {
	recorded, ok := s.tableSources[table]
	if !ok {
		return false
	}
	return sameSourceLocation(source, recorded)
}

// recordImportedTables attributes the tables a committed import created to the
// inputs that produced them, and takes the session bookkeeping with it.
//
// It runs only after the load has committed. Everything it writes — a table's
// source, its directory marker, its content baseline, a workbook's sheet record
// — is a claim about what the database holds, and making any of those before
// the commit would leave the session describing rows that were rolled back.
func (s *Shell) recordImportedTables(ctx context.Context, claims []claimedTables, after map[string]struct{}) []string {
	var imported []string
	for _, claim := range claims {
		owned := s.tablesNamedAfterFile(claim.target.loadPath, after)
		if len(owned) == 0 {
			// A re-import that overwrote tables it already owned creates no new
			// name to match, so the record is what says which tables are its.
			owned = s.tablesFromSource(claim.target.displayPath, after)
		}
		if len(owned) == 0 {
			continue
		}
		s.recordTableSources(ctx, owned, claim.target.displayPath)
		if claim.target.fromDirectory {
			for _, name := range owned {
				s.markDirImported(name)
			}
		} else {
			s.clearDirImported(owned)
		}
		s.warnKeywordTableNames(owned)
		imported = append(imported, owned...)
	}
	slices.Sort(imported)
	return imported
}

// importProducedNothing explains an import that committed without creating a
// table. A workbook whose only sheet has no cells arrives here, and saying
// "collision" would send the user looking for a second input that does not
// exist.
func importProducedNothing(plan *importPlan) error {
	names := make([]string, 0, len(plan.targets))
	for _, target := range plan.targets {
		names = append(names, target.displayPath)
	}
	if len(names) == 1 {
		return fmt.Errorf("%s produced no table; the file has no rows to import", names[0])
	}
	return fmt.Errorf("%s produced no table; none of them has rows to import", strings.Join(names, ", "))
}
