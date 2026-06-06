package shell

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"testing/quick"

	"github.com/nao1215/sqly/domain/model"
)

// randCell returns a small random string covering digits, letters, the empty
// string, and a few JSON/Unicode-significant runes, to vary keyed-row values.
func randCell(r *rand.Rand) string {
	alphabet := []rune("abc012 ,\"'\n日本é")
	n := r.Intn(6)
	out := make([]rune, 0, n)
	for range n {
		out = append(out, alphabet[r.Intn(len(alphabet))])
	}
	return string(out)
}

func featureQuickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 300,
		Rand:     rand.New(rand.NewSource(7)), //nolint:gosec // deterministic test seed
	}
}

// genColumns builds a schema with unique column names (real table columns are
// always unique) and random types, so the schema-diff properties exercise
// realistic inputs rather than degenerate duplicate-name cases.
func genColumns(r *rand.Rand) []inspectColumn {
	types := []string{"INTEGER", "TEXT", "REAL", "BLOB", "NUMERIC", ""}
	n := r.Intn(6)
	cols := make([]inspectColumn, 0, n)
	for i := range n {
		cols = append(cols, inspectColumn{
			Name: fmt.Sprintf("c%d", i),
			Type: types[r.Intn(len(types))],
		})
	}
	return cols
}

// TestCompareSchemas_ReflexiveProperty asserts that comparing any schema with
// itself reports no differences. This metamorphic relation (compare(x,x) is
// empty) guards the schema diff against spurious change reports.
func TestCompareSchemas_ReflexiveProperty(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(17)) //nolint:gosec // deterministic test seed
	for range 300 {
		cols := genColumns(r)
		got := compareSchemas(cols, cols)
		if !got.Equal || len(got.LeftOnlyColumns) != 0 ||
			len(got.RightOnlyColumns) != 0 || len(got.TypeChanges) != 0 {
			t.Fatalf("compare(x,x) reported a difference for %+v: %+v", cols, got)
		}
	}
}

// TestCompareSchemas_SwapProperty asserts that swapping the two sides swaps the
// left-only and right-only column sets and preserves the equality verdict.
func TestCompareSchemas_SwapProperty(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(19)) //nolint:gosec // deterministic test seed
	for range 300 {
		a := genColumns(r)
		b := genColumns(r)
		ab := compareSchemas(a, b)
		ba := compareSchemas(b, a)
		if ab.Equal != ba.Equal {
			t.Fatalf("equality verdict not symmetric for %+v vs %+v", a, b)
		}
		if !equalStringSet(ab.LeftOnlyColumns, ba.RightOnlyColumns) ||
			!equalStringSet(ab.RightOnlyColumns, ba.LeftOnlyColumns) {
			t.Fatalf("left/right-only sets not swapped: %+v vs %+v", ab, ba)
		}
	}
}

// genKeyedTable builds a table with a unique integer "id" key column and one
// random "v" value column, suitable for keyed-row comparison properties.
func genKeyedTable(r *rand.Rand) *model.Table {
	n := r.Intn(6)
	records := make([]model.Record, 0, n)
	for i := range n {
		records = append(records, model.Record{strconv.Itoa(i), randCell(r)})
	}
	return model.NewTable("t", model.Header{"id", "v"}, records)
}

// TestCompareKeyedRows_IdentityProperty asserts that comparing a keyed table with
// itself yields no added, removed, or modified rows.
func TestCompareKeyedRows_IdentityProperty(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(11)) //nolint:gosec // deterministic test seed
	for range 300 {
		tbl := genKeyedTable(r)
		rows, err := compareKeyedRows("t", "t", tbl, tbl, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows.Added) != 0 || len(rows.Removed) != 0 || len(rows.Modified) != 0 {
			t.Fatalf("compare(x,x) reported diffs: %+v", rows)
		}
	}
}

// TestCompareKeyedRows_SwapProperty asserts that swapping the two tables swaps
// added and removed keys and preserves the modified key set.
func TestCompareKeyedRows_SwapProperty(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(13)) //nolint:gosec // deterministic test seed
	keysOf := func(rows []compareRow, key string) []string {
		out := make([]string, 0, len(rows))
		for _, row := range rows {
			if v := row[key]; v != nil {
				out = append(out, *v)
			}
		}
		sort.Strings(out)
		return out
	}
	modKeys := func(rows []compareModifiedRow) []string {
		out := make([]string, 0, len(rows))
		for _, m := range rows {
			out = append(out, m.Key)
		}
		sort.Strings(out)
		return out
	}
	for range 300 {
		left := genKeyedTable(r)
		right := genKeyedTable(r)
		lr, err := compareKeyedRows("l", "r", left, right, "id")
		if err != nil {
			t.Fatal(err)
		}
		rl, err := compareKeyedRows("r", "l", right, left, "id")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(keysOf(lr.Added, "id"), keysOf(rl.Removed, "id")) {
			t.Fatalf("added(l,r) != removed(r,l): %v vs %v", keysOf(lr.Added, "id"), keysOf(rl.Removed, "id"))
		}
		if !reflect.DeepEqual(keysOf(lr.Removed, "id"), keysOf(rl.Added, "id")) {
			t.Fatalf("removed(l,r) != added(r,l)")
		}
		if !reflect.DeepEqual(modKeys(lr.Modified), modKeys(rl.Modified)) {
			t.Fatalf("modified keys differ under swap: %v vs %v", modKeys(lr.Modified), modKeys(rl.Modified))
		}
	}
}

// TestProfileColumnStats_PartitionProperty asserts the column counts partition
// the values exactly: every value is counted as null, blank, or non-empty, and
// the numeric and distinct counts never exceed the non-empty count.
func TestProfileColumnStats_PartitionProperty(t *testing.T) {
	t.Parallel()
	property := func(values []string, nulls []bool) bool {
		pc := profileColumnStats("c", "TEXT", values, nulls)
		var nullN, blankN, nonEmpty int64
		for i, v := range values {
			switch {
			case i < len(nulls) && nulls[i]:
				nullN++
			case v == "":
				blankN++
			default:
				nonEmpty++
			}
		}
		if pc.NullCount != nullN || pc.BlankCount != blankN {
			return false
		}
		if pc.NumericCount > nonEmpty || pc.DistinctCount > nonEmpty {
			return false
		}
		return pc.NullCount+pc.BlankCount+nonEmpty == int64(len(values))
	}
	if err := quick.Check(property, featureQuickConfig()); err != nil {
		t.Error(err)
	}
}

// equalStringSet reports whether two string slices contain the same elements,
// ignoring order and duplicates.
func equalStringSet(a, b []string) bool {
	as := append([]string(nil), a...)
	bs := append([]string(nil), b...)
	sort.Strings(as)
	sort.Strings(bs)
	return reflect.DeepEqual(as, bs)
}
