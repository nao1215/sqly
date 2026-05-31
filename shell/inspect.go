package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/nao1215/sqly/config"
	"github.com/nao1215/sqly/domain/model"
)

// inspectSampleLimit caps how many sample rows the report includes per table.
// A small sample is enough to convey shape and values for query authoring.
const inspectSampleLimit = 5

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

// inspectReport is the top-level JSON contract produced by --inspect.
type inspectReport struct {
	Tables []inspectTable `json:"tables"`
}

// runInspect prints a machine-readable JSON report of the imported tables:
// names, source mapping, columns, row counts, and a small sample of rows. It is
// the non-interactive discovery path for scripts and LLMs, so JSON is the
// primary contract and the report is written to stdout.
func (s *Shell) runInspect(ctx context.Context) error {
	tables, err := s.usecases.metadata.TablesName(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	if len(tables) == 0 {
		return errors.New("no tables to inspect: provide input files or directories")
	}

	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name())
	}
	// Sort by name so the report is deterministic regardless of import order.
	sort.Strings(names)

	report := inspectReport{Tables: make([]inspectTable, 0, len(names))}
	for _, name := range names {
		entry, err := s.inspectTable(ctx, name)
		if err != nil {
			return err
		}
		report.Tables = append(report.Tables, entry)
	}

	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode inspect report: %w", err)
	}
	fmt.Fprintln(config.Stdout, string(encoded))
	return nil
}

// inspectTable builds the report entry for a single table.
func (s *Shell) inspectTable(ctx context.Context, name string) (inspectTable, error) {
	columns, err := s.inspectColumns(ctx, name)
	if err != nil {
		return inspectTable{}, err
	}

	rowCount, err := s.inspectRowCount(ctx, name)
	if err != nil {
		return inspectTable{}, err
	}

	sample, err := s.inspectSample(ctx, name)
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
	result := make([]inspectColumn, 0, len(cols.Records()))
	for _, rec := range cols.Records() {
		if len(rec) <= colPK {
			continue
		}
		result = append(result, inspectColumn{
			Name:       rec[colName],
			Type:       rec[colType],
			Nullable:   rec[colNotNull] == "0",
			PrimaryKey: rec[colPK] != "0",
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

// inspectSample returns up to inspectSampleLimit rows rendered as a JSON array,
// reusing the table JSON renderer so the sample matches sqly's query JSON
// (ordered keys, string values).
func (s *Shell) inspectSample(ctx context.Context, name string) (json.RawMessage, error) {
	quoted := s.usecases.importer.QuoteIdentifier(name)
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoted, inspectSampleLimit)
	table, err := s.usecases.query.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to sample rows of %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := table.Print(&buf, model.PrintModeJSON); err != nil {
		return nil, fmt.Errorf("failed to render sample rows of %s: %w", name, err)
	}
	return json.RawMessage(bytes.TrimSpace(buf.Bytes())), nil
}
