package filesql_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	filesql "github.com/nao1215/filesql"
)

// TestIssue218LargeIntegerPreserved is a regression guard for
// https://github.com/nao1215/sqly/issues/218. A CSV value larger than
// math.MaxInt64 (for example 11040320260000000000) used to be inferred as a
// REAL column, stored as float64, and rendered in scientific notation with
// precision loss. The filesql version bundled with sqly must keep such values
// as TEXT so the exact digits round-trip through a query.
func TestIssue218LargeIntegerPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "issue218.csv")
	content := "ctsn,pocode\n" +
		"11040320260000000000,100031464478\n" +
		"11040320260000000001,100031464478\n"
	if err := os.WriteFile(csvPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	ctx := context.Background()
	db, err := filesql.OpenContext(ctx, csvPath)
	if err != nil {
		t.Fatalf("OpenContext failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	var got string
	if err := db.QueryRowContext(ctx, `SELECT ctsn FROM issue218 ORDER BY ctsn LIMIT 1`).Scan(&got); err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if want := "11040320260000000000"; got != want {
		t.Errorf("ctsn = %q, want %q (scientific-notation regression of sqly#218)", got, want)
	}
}
