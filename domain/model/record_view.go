package model

// RecordView is a read-only view of one row's display strings.
//
// It exists because the zero-copy read path and a safe public API were in
// conflict. Handing out the Table's own Record ([]string) made every walk of a
// result a way to corrupt it:
//
//	row, _ := table.Row(0)
//	row[0] = "corrupted"
//
// That is worse than ordinary aliasing here. A query result's strings are
// derived from its native cells, and the JSON and Parquet writers read those
// cells, so an in-place edit made the same table print "corrupted" as CSV and
// the original value as JSON, with nothing able to say which was right.
// Copying every row instead would have made walking a million-row result cost a
// million allocations.
//
// A view resolves both: it borrows the row without exposing it. The methods
// read, and there is no method that writes, so "do not modify this" is enforced
// by the type rather than asked for in a comment.
//
// A view is only valid while the Table it came from is alive; it holds no copy.
type RecordView struct {
	// record is never returned. Every accessor reads through it.
	record Record
}

// newRecordView wraps a row for reading. It is unexported because a view must
// always borrow a Table's storage, never a caller's slice.
func newRecordView(record Record) RecordView {
	return RecordView{record: record}
}

// Len returns the number of columns in the row.
func (v RecordView) Len() int {
	return len(v.record)
}

// At returns the display string of column i.
//
// An index outside the row returns the empty string rather than panicking. A
// Table built from strings may hold rows shorter than its header — an imported
// file with a ragged tail, a synthesized report — and every formatter already
// treats a missing cell as blank. Making that a panic would turn a data problem
// into a crash.
func (v RecordView) At(i int) string {
	if i < 0 || i >= len(v.record) {
		return ""
	}
	return v.record[i]
}

// AppendTo appends the row's values to dst and returns the result. It is the
// bridge to APIs that need a real []string — encoding/csv's Writer, for one —
// without the view surrendering its own storage. Passing a reused buffer keeps
// a row-by-row writer allocation-free:
//
//	buf := make([]string, 0, table.ColumnCount())
//	for _, row := range table.Rows {
//	    buf = row.AppendTo(buf[:0])
//	    w.Write(buf)
//	}
func (v RecordView) AppendTo(dst []string) []string {
	return append(dst, v.record...)
}

// Record returns a copy of the row as a Record. Use it when a caller needs to
// keep or modify the values; the copy is why doing so is safe.
func (v RecordView) Record() Record {
	if v.record == nil {
		return nil
	}
	return append(make(Record, 0, len(v.record)), v.record...)
}
