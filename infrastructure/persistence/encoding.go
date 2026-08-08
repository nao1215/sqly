package persistence

import (
	"fmt"
	"io"

	"github.com/nao1215/sqly/domain/model"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// An export wrote UTF-8 whatever the source was, so a write-back over a file
// read with --encoding quietly changed the file's encoding on disk, and the
// caller's own next run of the same command decoded the new bytes as the old
// encoding and returned mojibake. Compression has always been preserved by a
// write-back; this is the same promise for text.
//
// A rune the target encoding has no way to write fails the export rather than
// being replaced. The x/text encoders already refuse rather than substitute, and
// they are deliberately not wrapped in encoding.ReplaceUnsupported: a
// substitution is the silent corruption the import side refuses, and it would be
// worse here, where the destination is a file the user already had.

// NewEncodingWriter wraps w so text written to it is encoded as enc, and returns
// the finish that flushes the encoder. For UTF-8 it returns w unchanged and a
// finish that does nothing: the values are already UTF-8, so a transformer would
// only copy them.
func NewEncodingWriter(w io.Writer, enc model.TextEncoding) (io.Writer, func() error) {
	encoder, ok := exportEncoder(enc)
	if !ok {
		return w, func() error { return nil }
	}
	tw := transform.NewWriter(w, encoder)
	finish := func() error {
		if err := tw.Close(); err != nil {
			return encodingWriteError(enc, err)
		}
		return nil
	}
	return &encodingErrorWriter{writer: tw, encoding: enc}, finish
}

// encodingErrorWriter names the encoding in a failure the encoder produced, so
// the message says which encoding could not write the value rather than
// reporting an unattributed transform error.
type encodingErrorWriter struct {
	writer   io.Writer
	encoding model.TextEncoding
}

func (w *encodingErrorWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		return n, encodingWriteError(w.encoding, err)
	}
	return n, nil
}

// exportEncoder returns the transformer that writes enc, and whether enc needs
// one. The UTF-16 encoders write a byte-order mark, because that is what lets a
// later read recognize the file without being told the encoding.
func exportEncoder(enc model.TextEncoding) (transform.Transformer, bool) {
	var target encoding.Encoding
	switch enc {
	case model.TextEncodingShiftJIS:
		target = japanese.ShiftJIS
	case model.TextEncodingEUCJP:
		target = japanese.EUCJP
	case model.TextEncodingISO2022JP:
		target = japanese.ISO2022JP
	case model.TextEncodingUTF16LE:
		target = unicode.UTF16(unicode.LittleEndian, unicode.UseBOM)
	case model.TextEncodingUTF16BE:
		target = unicode.UTF16(unicode.BigEndian, unicode.UseBOM)
	default:
		return nil, false
	}
	return target.NewEncoder(), true
}

// encodingWriteError words the refusal. It names the encoding because that is
// what has to change: the value is fine, and the encoding the file is written in
// is what cannot hold it.
func encodingWriteError(enc model.TextEncoding, err error) error {
	return fmt.Errorf("this table holds a value %s cannot write, so the file would lose it: %w", enc, err)
}
