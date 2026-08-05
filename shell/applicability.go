package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nao1215/sqly/domain/model"
)

// An import option that only affects some formats is a promise about how this
// run reads its inputs. When the run has no input that option could ever touch,
// the promise is empty: the flag was typed, accepted, and then had no effect at
// all. Reporting that as success is what makes a CLI untrustworthy in a script —
// `--encoding shift-jis` silently doing nothing is indistinguishable from
// `--encoding shift-jis` working, until the mojibake shows up downstream.
//
// So a flag the user typed must be able to apply to at least one input. The
// checks below run before anything is imported, and they only reject a run where
// the option can be proven irrelevant: a directory or a URL might hold anything,
// so it counts as applicable and the run proceeds.

// textImportExtensions are the formats sqly decodes as text, and so the ones
// --encoding can affect. Every other format either carries its own encoding
// (Excel, Parquet) or is defined as ASCII (ACH, Fedwire).
var textImportExtensions = map[string]bool{
	model.ExtCSV:   true,
	model.ExtTSV:   true,
	model.ExtLTSV:  true,
	model.ExtJSON:  true,
	model.ExtJSONL: true,
}

// delimitedImportExtensions are the formats with a header row and a fixed field
// count per row, and so the only ones --row-mismatch can affect.
var delimitedImportExtensions = map[string]bool{
	model.ExtCSV: true,
	model.ExtTSV: true,
}

// excelImportExtensions is the one format that has sheets, and so the only one
// --include-hidden-sheets can affect.
var excelImportExtensions = map[string]bool{
	model.ExtExcel: true,
}

// validateOptionApplicability rejects an import option the user typed that
// cannot apply to any input of this run. It runs before the import so a rejected
// run reads no file and writes nothing.
func (s *Shell) validateOptionApplicability() error {
	if s.argument.IsExplicit("encoding") && !s.hasInputMatching(textImportExtensions) {
		return &invocationError{Err: fmt.Errorf("--encoding %s applies to csv, tsv, ltsv, json, and jsonl inputs, and this run has none; drop the flag",
			s.argument.Encoding)}
	}
	if s.argument.IsExplicit("row-mismatch") && !s.hasInputMatching(delimitedImportExtensions) {
		return &invocationError{Err: fmt.Errorf("--row-mismatch %s applies to csv and tsv inputs, and this run has none; drop the flag",
			s.argument.RowMismatch)}
	}
	// --include-hidden-sheets is checked last and one step more leniently than
	// the others, because it is not only an import option: it is the session's
	// sheet policy, and an interactive shell started with no input at all can
	// still .import a workbook later. Only a run that already knows every input
	// it will ever read, and knows none of them is a workbook, can prove the flag
	// useless.
	if s.argument.IsExplicit("include-hidden-sheets") && s.hasAnyInput() && !s.hasInputMatching(excelImportExtensions) {
		return &invocationError{Err: errors.New("--include-hidden-sheets applies to xlsx inputs, and this run has none; drop the flag")}
	}
	return nil
}

// hasAnyInput reports whether the run was given something to import up front. A
// run with nothing has not decided its inputs yet: the shell it starts reads
// them from .import.
func (s *Shell) hasAnyInput() bool {
	return s.argument.StdinFormat != "" || len(s.argument.FilePaths) > 0
}

// hasInputMatching reports whether any input of this run could be one of the
// given formats. A directory or a URL is counted as a match: its contents are
// not known until the import runs, and rejecting a run over what a directory
// might not contain would be worse than letting an option go unused there.
func (s *Shell) hasInputMatching(extensions map[string]bool) bool {
	if s.argument.StdinFormat != "" {
		if ext, ok := model.StdinFormatExtension(s.argument.StdinFormat); ok && extensions[ext] {
			return true
		}
	}
	for _, path := range s.argument.FilePaths {
		if isRemoteURL(path) {
			return true // the server decides the format; the import reports a mismatch
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return true // unknown contents, or a path the import step will report
		}
		if extensions[importExtension(path)] {
			return true
		}
	}
	return false
}

// importExtension returns the format extension of an input path, with any
// compression suffix removed, lowercased. "sales.CSV.gz" yields ".csv".
func importExtension(path string) string {
	base := path
	if _, ok := model.CompressionFromExtension(strings.ToLower(filepath.Ext(base))); ok {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return strings.ToLower(filepath.Ext(base))
}
