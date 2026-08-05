package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// inspectColumn describes one column in the inspect report.
type inspectColumn struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	PrimaryKey bool   `json:"primary_key"`
}

// inspectTable describes one imported table in the inspect report.
type inspectTable struct {
	Name       string          `json:"name"`
	Source     string          `json:"source,omitempty"`
	RowCount   int64           `json:"row_count"`
	Columns    []inspectColumn `json:"columns"`
	SampleRows json.RawMessage `json:"sample_rows"`
}

// inspectExcelSheet describes one sheet of an imported workbook: whether the
// workbook showed it, whether this run imported it, and what table it became.
//
// It exists because a workbook is the one input whose tables are not the whole
// story. sqly imports only the sheets a workbook shows unless told otherwise,
// so "which tables did I get?" leaves "and what else was in there?" unanswered,
// and the answer is not derivable from the file name or the table list.
//
// Table is omitted for a sheet that was not imported: there is no table to
// name, and an empty string would read as one.
type inspectExcelSheet struct {
	Source   string `json:"source"`
	Name     string `json:"name"`
	Visible  bool   `json:"visible"`
	Imported bool   `json:"imported"`
	Table    string `json:"table,omitempty"`
}

// InspectSchemaVersion is the version of the --inspect JSON contract. It is the
// document's own version, not sqly's: it changes when the shape of the JSON
// changes in a way a consumer cannot absorb, and stays put for a change a
// consumer that ignores unknown fields already handles.
//
// The policy that decides which is which is documented at
// https://nao1215.github.io/sqly/reference/#inspect-json-schema and is checked
// against this constant, against the published JSON Schema, and against the
// reference page by TestInspect_SchemaVersionAgreesEverywhere.
const InspectSchemaVersion = 1

// inspectReport is the top-level JSON contract produced by --inspect.
//
// Field order here is the order the JSON document carries, because
// encoding/json writes struct fields in declaration order. It is fixed so the
// same binary given the same inputs and options produces the same bytes.
//
// SchemaVersion and SqlyVersion answer two different questions and are easy to
// confuse. SchemaVersion says how to read this document; SqlyVersion says which
// binary wrote it. A consumer branches on the first and reports the second.
//
// ExcelSheets is additive: it is absent for a run with no workbook among its
// inputs, so a consumer reading only Tables sees exactly what it saw before.
type inspectReport struct {
	SchemaVersion int                 `json:"schema_version"`
	SqlyVersion   string              `json:"sqly_version"`
	Tables        []inspectTable      `json:"tables"`
	ExcelSheets   []inspectExcelSheet `json:"excel_sheets,omitempty"`
}

func outputModeFlagName(o *config.Output) string {
	if o == nil {
		return ""
	}
	return o.Mode.String()
}

// validateInspectFlags rejects --inspect combined with other effectful flags.
// --inspect is a self-contained discovery path that imports inputs, prints a
// JSON report, and exits, so flags that ask for a different action (--sql,
// --sql-file) or a side effect (--output) would otherwise be
// silently discarded. Failing fast keeps the contract explicit for scripts.
func (s *Shell) validateInspectFlags() error {
	if !s.argument.InspectFlag {
		return nil
	}
	switch {
	case s.argument.Query != "":
		return &invocationError{Err: errors.New("--inspect cannot be combined with --sql")}
	case s.argument.SQLFilePath != "":
		return &invocationError{Err: errors.New("--inspect cannot be combined with --sql-file")}
	case s.argument.Output.FilePath != "":
		return &invocationError{Err: errors.New("--inspect cannot be combined with --output")}
	// --output-format selects a result format, but --inspect always emits its
	// own JSON report. Reject the conflicting flag instead of silently discarding
	// it, matching the other --inspect conflict checks. What counts is that the
	// user wrote it: --output-format table is discarded exactly as much as
	// --output-format csv is, and only the default nobody asked for is silent.
	case s.argument.IsExplicit("output-format"):
		return &invocationError{Err: fmt.Errorf("--inspect cannot be combined with --output-format %s", outputModeFlagName(s.argument.Output))}
	}
	return nil
}

// runInspect prints a machine-readable JSON report of the imported tables:
// names, source mapping, columns, row counts, and — only when asked — a small
// sample of rows. It is the non-interactive discovery path for scripts and LLMs,
// so JSON is the primary contract and the report is written to stdout.
//
// The report is built whole and written in one call. Nothing reaches stdout
// until every table has been read, so a failure part-way leaves stdout empty
// rather than holding half a JSON document a consumer would have to guess at.
func (s *Shell) runInspect(ctx context.Context) error {
	// A negative count cannot reach here: config rejects it while parsing, as a
	// usage error, before any input is read.
	sampleLimit := s.argument.InspectSample

	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		// Nothing was inspected because nothing was given to inspect, so the fix is
		// on the command line. Reporting this as a failed statement told a wrapper
		// to look at SQL that never ran — the case an agent lands in when it
		// expands an empty file list into `sqly --inspect $FILES`.
		return &invocationError{Err: errors.New("no tables to inspect: provide input files or directories")}
	}

	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	// Sort by name so the report is deterministic regardless of import order.
	slices.Sort(names)

	report := inspectReport{
		SchemaVersion: InspectSchemaVersion,
		// The same version --version prints, from the same accessor: a release
		// binary reports the tag its ldflags carried, a `go install` build reports
		// the module version, and a local build reports "(devel)". Nothing here
		// substitutes a fixed string, so a report can always be traced to the
		// binary that wrote it.
		SqlyVersion: config.GetVersion(),
		Tables:      make([]inspectTable, 0, len(names)),
	}
	for _, name := range names {
		entry, err := s.inspectTable(ctx, name, sampleLimit)
		if err != nil {
			return err
		}
		report.Tables = append(report.Tables, entry)
	}
	report.ExcelSheets = s.excelSheetReports()

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode inspect report: %w", err)
	}
	fmt.Fprintln(config.Stdout, string(encoded))
	return nil
}

// inspectTable builds the report entry for a single table. sampleLimit caps the
// sample rows; 0 means schema-only.
func (s *Shell) inspectTable(ctx context.Context, name string, sampleLimit int) (inspectTable, error) {
	columns, err := s.inspectColumns(ctx, name)
	if err != nil {
		return inspectTable{}, err
	}

	rowCount, err := s.inspectRowCount(ctx, name)
	if err != nil {
		return inspectTable{}, err
	}

	sample, err := s.inspectSample(ctx, name, sampleLimit)
	if err != nil {
		return inspectTable{}, err
	}

	return inspectTable{
		Name:       name,
		Source:     s.tableSources[name],
		RowCount:   rowCount,
		Columns:    columns,
		SampleRows: sample,
	}, nil
}

// inspectColumns returns column metadata using the same PRAGMA table_info path
// as .describe, preserving definition order.
func (s *Shell) inspectColumns(ctx context.Context, name string) ([]inspectColumn, error) {
	cols, err := s.tableColumns(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to read columns of %s: %w", name, err)
	}
	// PRAGMA table_info columns: cid, name, type, notnull, dflt_value, pk.
	const (
		colName    = 1
		colType    = 2
		colNotNull = 3
		colPK      = 5
	)
	result := make([]inspectColumn, 0, cols.RowCount())
	for _, rec := range cols.Rows {
		if rec.Len() <= colPK {
			continue
		}
		result = append(result, inspectColumn{
			Name:       rec.At(colName),
			Type:       rec.At(colType),
			Nullable:   rec.At(colNotNull) == "0",
			PrimaryKey: rec.At(colPK) != "0",
		})
	}
	return result, nil
}

// inspectRowCount returns the number of rows in the table.
func (s *Shell) inspectRowCount(ctx context.Context, name string) (int64, error) {
	quoted := s.usecases.importer.QuoteIdentifier(name)
	table, err := s.usecases.query.Query(ctx, "SELECT COUNT(*) FROM "+quoted)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows of %s: %w", name, err)
	}
	records := table.Records()
	if len(records) == 0 || len(records[0]) == 0 {
		return 0, nil
	}
	count, err := strconv.ParseInt(records[0][0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected row count for %s: %w", name, err)
	}
	return count, nil
}

// inspectSample returns up to limit rows rendered as a JSON array, reusing the
// table JSON renderer so the sample matches sqly's query JSON (ordered keys,
// string values). A limit of 0 returns an empty array without querying.
func (s *Shell) inspectSample(ctx context.Context, name string, limit int) (json.RawMessage, error) {
	if limit == 0 {
		return json.RawMessage("[]"), nil
	}
	quoted := s.usecases.importer.QuoteIdentifier(name)
	// The sample is the first rows of the file, and it says so: an unordered
	// LIMIT is whatever the scan happens to produce, which SQLite is free to
	// change. rowid is the import order for every table sqly creates from a file,
	// so ordering by it makes "the first rows" a promise rather than an
	// observation. A table without a rowid falls back to the plain scan.
	table, err := s.usecases.query.Query(ctx,
		fmt.Sprintf("SELECT * FROM %s ORDER BY rowid LIMIT %d", quoted, limit))
	if err != nil {
		table, err = s.usecases.query.Query(ctx,
			fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoted, limit))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to sample rows of %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := table.Print(&buf, model.PrintModeJSON); err != nil {
		return nil, fmt.Errorf("failed to render sample rows of %s: %w", name, err)
	}
	return json.RawMessage(bytes.TrimSpace(buf.Bytes())), nil
}
