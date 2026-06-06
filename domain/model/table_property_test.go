package model

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

// quickConfig keeps property runs deterministic and reasonably sized.
func quickConfig() *quick.Config {
	return &quick.Config{
		MaxCount: 300,
		Rand:     rand.New(rand.NewSource(1)), //nolint:gosec // deterministic test seed
	}
}

// genTable is a quick.Generator that produces a Table with unique column names
// (so JSON objects keyed by column round-trip without collisions) and arbitrary
// string cell values, including characters that require JSON escaping.
type genTable struct {
	table *Table
}

// randCell returns a random string covering ASCII, whitespace, JSON-significant
// characters, and a few multibyte runes, to stress format escaping.
func randCell(r *rand.Rand) string {
	alphabet := []rune(`abc012 ,	"'\` + "\n" + `日本語é`)
	n := r.Intn(8)
	var b strings.Builder
	for range n {
		b.WriteRune(alphabet[r.Intn(len(alphabet))])
	}
	return b.String()
}

// Generate implements quick.Generator.
func (genTable) Generate(r *rand.Rand, _ int) reflect.Value {
	cols := r.Intn(5) + 1
	rows := r.Intn(6)

	header := make(Header, cols)
	for i := range cols {
		// Prefix with the index to guarantee uniqueness; suffix with a random
		// cell so escaping of column names is exercised too.
		header[i] = "c" + string(rune('0'+i)) + "_" + randCell(r)
	}

	records := make([]Record, rows)
	for i := range records {
		rec := make(Record, cols)
		for j := range cols {
			rec[j] = randCell(r)
		}
		records[i] = rec
	}
	return reflect.ValueOf(genTable{table: NewTable("t", header, records)})
}

// rowMaps returns the table's records as []map[col]value for comparison.
func rowMaps(t *Table) []map[string]string {
	out := make([]map[string]string, 0, len(t.Records()))
	for _, rec := range t.Records() {
		m := make(map[string]string, len(t.Header()))
		for i, h := range t.Header() {
			m[h] = rec[i]
		}
		out = append(out, m)
	}
	return out
}

// TestTable_JSONRoundTripProperty asserts that any table rendered as a JSON
// array decodes back to the same column/value pairs. This metamorphic relation
// (render -> parse == identity) guards the JSON writer against escaping and
// ordering regressions.
func TestTable_JSONRoundTripProperty(t *testing.T) {
	property := func(g genTable) bool {
		var buf bytes.Buffer
		if err := g.table.Print(&buf, PrintModeJSON); err != nil {
			return false
		}
		var got []map[string]string
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			return false
		}
		want := rowMaps(g.table)
		if len(want) == 0 {
			return len(got) == 0 // empty result decodes as []
		}
		return reflect.DeepEqual(got, want)
	}
	if err := quick.Check(property, quickConfig()); err != nil {
		t.Error(err)
	}
}

// FuzzJSONScalarToken asserts the typed-mode scalar tokenizer never emits a
// byte sequence that is not valid JSON, for any input string. A value that is
// not a canonical number or boolean must come back as a JSON string equal to the
// input, and a canonical number/bool must decode back to the same text.
func FuzzJSONScalarToken(f *testing.F) {
	for _, s := range []string{"", "0", "-1.5", "1e10", "007", "true", "false", "hello", "\"q\"", "\n\t", "日本語", "1.2.3", "+1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		tok, err := jsonScalarToken(s)
		if err != nil {
			t.Fatalf("jsonScalarToken(%q) returned error: %v", s, err)
		}
		// Invariant 1: the token is always valid JSON, for any input.
		if !json.Valid(tok) {
			t.Fatalf("jsonScalarToken(%q) produced invalid JSON: %q", s, tok)
		}
		// Invariant 2: the token matches the exact typed contract. Canonical
		// numbers and the JSON booleans are emitted verbatim (lossless); every
		// other value falls back to json.Marshal, identical to the legacy string
		// contract (which is lossy for invalid UTF-8 the same way encoding/json is).
		var want []byte
		switch {
		case s == "true" || s == "false":
			want = []byte(s)
		case isCanonicalJSONNumber(s):
			want = []byte(s)
		default:
			want, err = json.Marshal(s)
			if err != nil {
				t.Fatalf("json.Marshal(%q) returned error: %v", s, err)
			}
		}
		if !bytes.Equal(tok, want) {
			t.Fatalf("jsonScalarToken(%q) = %q, want %q", s, tok, want)
		}
	})
}

// TestTable_TypedJSONRoundTripProperty asserts the typed JSON contract never
// corrupts data: for any table, typed output is valid JSON, and every cell
// decodes back to its original string. A canonical number decodes to a
// json.Number whose text equals the original (lossless, no scientific notation),
// "true"/"false" decode to the matching boolean, and any other value stays a
// string. This is the metamorphic relation render(typed) -> parse -> stringify
// == identity.
func TestTable_TypedJSONRoundTripProperty(t *testing.T) {
	stringify := func(v any) (string, bool) {
		switch x := v.(type) {
		case json.Number:
			return x.String(), true
		case bool:
			if x {
				return "true", true
			}
			return "false", true
		case string:
			return x, true
		default:
			return "", false
		}
	}
	property := func(g genTable) bool {
		g.table.SetJSONTyped(true)
		var buf bytes.Buffer
		if err := g.table.Print(&buf, PrintModeJSON); err != nil {
			return false
		}
		dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
		dec.UseNumber()
		var got []map[string]any
		if err := dec.Decode(&got); err != nil {
			return false
		}
		want := rowMaps(g.table)
		if len(want) != len(got) {
			return false
		}
		for i, row := range got {
			if len(row) != len(want[i]) {
				return false
			}
			for k, v := range row {
				s, ok := stringify(v)
				if !ok || s != want[i][k] {
					return false
				}
			}
		}
		return true
	}
	if err := quick.Check(property, quickConfig()); err != nil {
		t.Error(err)
	}
}

// TestTable_NDJSONRoundTripProperty asserts every NDJSON line decodes back to
// the corresponding record, and the line count matches the record count.
func TestTable_NDJSONRoundTripProperty(t *testing.T) {
	property := func(g genTable) bool {
		var buf bytes.Buffer
		if err := g.table.Print(&buf, PrintModeNDJSON); err != nil {
			return false
		}
		out := buf.String()
		if len(g.table.Records()) == 0 {
			return out == "" // empty result prints nothing
		}
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		if len(lines) != len(g.table.Records()) {
			return false
		}
		want := rowMaps(g.table)
		for i, line := range lines {
			var got map[string]string
			if err := json.Unmarshal([]byte(line), &got); err != nil {
				return false
			}
			if !reflect.DeepEqual(got, want[i]) {
				return false
			}
		}
		return true
	}
	if err := quick.Check(property, quickConfig()); err != nil {
		t.Error(err)
	}
}

// TestTable_EqualReflexiveSymmetricProperty checks the Equal contract: a table
// equals itself, and Equal is symmetric for any pair of generated tables.
func TestTable_EqualReflexiveSymmetricProperty(t *testing.T) {
	reflexive := func(g genTable) bool {
		return g.table.Equal(g.table)
	}
	if err := quick.Check(reflexive, quickConfig()); err != nil {
		t.Errorf("reflexivity: %v", err)
	}

	symmetric := func(a, b genTable) bool {
		return a.table.Equal(b.table) == b.table.Equal(a.table)
	}
	if err := quick.Check(symmetric, quickConfig()); err != nil {
		t.Errorf("symmetry: %v", err)
	}
}

// TestHeaderRecordEqual_ReflexiveProperty checks that Header.Equal and
// Record.Equal hold for identical copies of arbitrary string slices.
func TestHeaderRecordEqual_ReflexiveProperty(t *testing.T) {
	headerProp := func(s []string) bool {
		h := NewHeader(s)
		cp := NewHeader(append([]string(nil), s...))
		return h.Equal(cp) && cp.Equal(h)
	}
	if err := quick.Check(headerProp, quickConfig()); err != nil {
		t.Errorf("header: %v", err)
	}

	recordProp := func(s []string) bool {
		r := NewRecord(s)
		cp := NewRecord(append([]string(nil), s...))
		return r.Equal(cp) && cp.Equal(r)
	}
	if err := quick.Check(recordProp, quickConfig()); err != nil {
		t.Errorf("record: %v", err)
	}
}
