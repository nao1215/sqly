package model

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// RowSink takes a query result a row at a time: the column names once, then
// each row. The cells of a row are not valid past the call that hands them over.
type RowSink interface {
	Header(header []string) error
	Row(cells []Cell) error
}

// Streamable reports whether mode can be written a row at a time by a
// RowStream. The table, Markdown and vertical formats measure or align every
// row before the first is printed, and LTSV, Excel and Parquet have rules of
// their own, so they are printed from a Table.
func Streamable(mode PrintMode) bool {
	switch mode {
	case PrintModeCSV, PrintModeTSV, PrintModeJSON, PrintModeJSONL:
		return true
	default:
		return false
	}
}

// RowStream formats a query result a row at a time into the same bytes
// Table.Print writes for mode, without holding the result as a Table.
//
// Every row is formatted and checked as it arrives, into a buffer, and nothing
// reaches the destination until WriteTo is called after the last row. A value
// the format cannot hold fails the stream before the first byte is written, the
// guarantee Table.Print gives by checking every value first: a reader of stdout
// never takes a truncated document for a whole one. What the buffer holds is the
// output itself, which is far smaller than the values of every cell held for a
// Table.
type RowStream struct {
	mode   PrintMode
	format string
	header Header
	out    spool
	rows   int

	delimited *delimitedWriter
	display   []string

	keys [][]byte
	obj  bytes.Buffer
}

// NewRowStream returns a stream for mode, which must be Streamable.
func NewRowStream(mode PrintMode) (*RowStream, error) {
	s := &RowStream{mode: mode}
	switch mode {
	case PrintModeCSV:
		s.format = formatCSV
		s.delimited = newDelimitedWriter(&s.out, ',')
	case PrintModeTSV:
		s.format = formatTSV
		s.delimited = newDelimitedWriter(&s.out, '\t')
	case PrintModeJSON:
		s.format = formatJSON
	case PrintModeJSONL:
		s.format = formatJSONL
	default:
		return nil, errors.New("this output format cannot be written a row at a time")
	}
	return s, nil
}

// Header takes the column names. It is called once, before the first row.
func (s *RowStream) Header(header []string) error {
	s.header = append(Header(nil), header...)
	switch s.mode {
	case PrintModeCSV, PrintModeTSV:
		if err := EnsureHeaderReimportable(s.format, s.header); err != nil {
			return err
		}
		if err := s.delimited.write([]string(s.header)); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	default:
		if dup := duplicateName(s.header); dup != "" {
			return fmt.Errorf("%s output requires unique column names, but %q appears more than once; alias the duplicate columns", s.format, dup)
		}
		keys, err := jsonKeysOf(s.header)
		if err != nil {
			return err
		}
		s.keys = keys
	}
	return nil
}

// Row formats one row. cells is not kept past the call, so the caller may
// reuse it for the next row.
func (s *RowStream) Row(cells []Cell) error {
	if len(cells) != len(s.header) {
		return fmt.Errorf("%w: row %d has %d cells, header has %d columns", ErrCellShapeMismatch, s.rows, len(cells), len(s.header))
	}
	switch s.mode {
	case PrintModeCSV, PrintModeTSV:
		s.display = s.display[:0]
		for i, cell := range cells {
			value := cell.String()
			if err := ensureTextValueRepresentable(s.format, s.header[i], value); err != nil {
				return err
			}
			s.display = append(s.display, value)
		}
		if err := s.delimited.write(s.display); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	default:
		for i, cell := range cells {
			if err := jsonRenderableValue(cell.value); err != nil {
				return fmt.Errorf("failed to encode value for column %q: %w", s.header[i], err)
			}
		}
		if err := appendJSONObject(&s.obj, s.keys, s.header, func(i int) any { return cells[i].value }); err != nil {
			return err
		}
		prefix, suffix := "", "\n"
		if s.mode == PrintModeJSON {
			prefix, suffix = ",\n  ", ""
			if s.rows == 0 {
				prefix = "[\n  "
			}
		}
		if err := s.out.WriteString(prefix); err != nil {
			return err
		}
		if _, err := s.out.Write(s.obj.Bytes()); err != nil {
			return err
		}
		if err := s.out.WriteString(suffix); err != nil {
			return err
		}
	}
	s.rows++
	return nil
}

// Rows is the number of rows taken so far.
func (s *RowStream) Rows() int {
	return s.rows
}

// WriteTo ends the document, writes it to out and releases what the stream
// held. It is called once, after the last row.
func (s *RowStream) WriteTo(out io.Writer) (int64, error) {
	switch s.mode {
	case PrintModeCSV, PrintModeTSV:
		if err := s.delimited.flush(); err != nil {
			return 0, errors.Join(err, s.out.Close())
		}
	case PrintModeJSON:
		end := "\n]\n"
		if s.rows == 0 {
			end = "[]\n"
		}
		if err := s.out.WriteString(end); err != nil {
			return 0, errors.Join(err, s.out.Close())
		}
	}
	return s.out.WriteTo(out)
}

// Close releases what the stream holds. A stream that is not written out has
// to be closed, since its output may sit in a temporary file.
func (s *RowStream) Close() error {
	return s.out.Close()
}

// spoolMemoryLimit is how much output a spool keeps in memory before it moves
// to a temporary file.
const spoolMemoryLimit = 4 << 20

// spool holds a stream's output until it is written out: in memory while it is
// small, and in a temporary file once it outgrows spoolMemoryLimit, so a large
// result costs disk rather than the memory of a second copy of itself.
type spool struct {
	mem  bytes.Buffer
	file *os.File
	err  error
}

func (s *spool) Write(p []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.file == nil && s.mem.Len()+len(p) > spoolMemoryLimit {
		if err := s.spill(); err != nil {
			return 0, err
		}
	}
	if s.file == nil {
		return s.mem.Write(p)
	}
	n, err := s.file.Write(p)
	if err != nil {
		s.err = fmt.Errorf("failed to write the output to a temporary file: %w", err)
		return n, s.err
	}
	return n, nil
}

func (s *spool) WriteString(str string) error {
	_, err := s.Write([]byte(str))
	return err
}

// spill moves what is in memory to a temporary file.
func (s *spool) spill() error {
	f, err := os.CreateTemp("", "sqly-output-*")
	if err != nil {
		s.err = fmt.Errorf("failed to create a temporary file for the output: %w", err)
		return s.err
	}
	s.file = f
	if _, err := s.mem.WriteTo(f); err != nil {
		s.err = fmt.Errorf("failed to write the output to a temporary file: %w", err)
	}
	s.mem = bytes.Buffer{}
	return s.err
}

// WriteTo writes everything the spool holds to out and releases it.
func (s *spool) WriteTo(out io.Writer) (int64, error) {
	if s.err != nil {
		return 0, errors.Join(s.err, s.Close())
	}
	if s.file == nil {
		return s.mem.WriteTo(out)
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return 0, errors.Join(err, s.Close())
	}
	n, err := io.Copy(out, s.file)
	return n, errors.Join(err, s.Close())
}

// Close removes the temporary file, if there is one.
func (s *spool) Close() error {
	if s.file == nil {
		return nil
	}
	name := s.file.Name()
	err := errors.Join(s.file.Close(), os.Remove(name))
	s.file = nil
	return err
}
