package model

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// ErrCellShapeMismatch is returned when a row of cells does not have one cell
// per header column. A query result whose row width disagrees with its header
// cannot be rendered by any output format, and discovering that inside a
// formatter would mean a partially written stream, so the mismatch is rejected
// when the Table is built.
var ErrCellShapeMismatch = errors.New("table: row width does not match the header")

// Cell is one value of a query result.
//
// A Cell keeps the database driver's native scalar, and every representation
// sqly prints is derived from it: the display string used by table, CSV, TSV,
// LTSV, and Markdown output, and the JSON token used by JSON and NDJSON output.
// Because there is exactly one source of truth per cell, the string a user reads
// and the scalar a JSON consumer decodes cannot drift apart — which is what a
// separate display slice and a separate "native values" slice, kept in step by
// hand, could not guarantee.
//
// SQL NULL is represented by a nil value. That is not a convention imposed on
// top of the driver: database/sql scans NULL as an untyped nil and never scans a
// non-NULL value as one, so "nil means NULL" is the driver's own contract. An
// empty string is therefore a TEXT value distinct from NULL.
type Cell struct {
	// value is the driver's scalar for this cell: int64, float64, bool, string,
	// []byte, time.Time, or nil for SQL NULL.
	value any
}

// NewCell returns a Cell holding the driver value v. A []byte is copied because
// database/sql documents that the memory backing a scanned []byte is only valid
// until the next call to Rows.Next; keeping the caller's slice would let a later
// row rewrite an earlier cell.
func NewCell(v any) Cell {
	if raw, ok := v.([]byte); ok {
		return Cell{value: append([]byte(nil), raw...)}
	}
	return Cell{value: v}
}

// IsNull reports whether the cell is SQL NULL.
func (c Cell) IsNull() bool {
	return c.value == nil
}

// Value returns the driver's native value, or nil for SQL NULL. A []byte is
// returned as a copy so a caller cannot mutate the Table's contents through it.
func (c Cell) Value() any {
	if raw, ok := c.value.([]byte); ok {
		return append([]byte(nil), raw...)
	}
	return c.value
}

// String returns the display value used by every text output format. A NULL
// renders as the empty string, matching the long-standing table/CSV rendering;
// the JSON formats consult IsNull instead so they can emit a real null.
//
// The formatting is deliberately the driver-neutral one: an int64 prints as its
// decimal digits, a float64 in Go's shortest round-trip form, and a []byte as
// its bytes interpreted as text. TEXT is returned verbatim, so "00123" keeps its
// leading zeros and "123" is never re-read as a number.
func (c Cell) String() string {
	switch v := c.value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []byte:
		return string(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return formatFloat(v, 64)
	case float32:
		// The value's own width, not float64's: formatting a float32 as 64 bits
		// prints the error its conversion introduced (1.1 becomes
		// 1.100000023841858).
		return formatFloat(float64(v), 32)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// The three values that have no numeric spelling, written the same way wherever
// they appear: the JSON formats quote these words, the text formats print them
// bare. Defined once so the two cannot drift apart.
const (
	infinityToken    = "Infinity"
	negInfinityToken = "-Infinity"
	notANumberToken  = "NaN"
)

// formatFloat returns the shortest form that round-trips at bitSize, except for
// the three values above.
func formatFloat(f float64, bitSize int) string {
	switch {
	case math.IsInf(f, 1):
		return infinityToken
	case math.IsInf(f, -1):
		return negInfinityToken
	case math.IsNaN(f):
		return notANumberToken
	}
	return strconv.FormatFloat(f, 'g', -1, bitSize)
}
