package shell

import (
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/nao1215/sqly/domain/model"
)

// A text input that is not UTF-8 is refused, and naming a legacy encoding used
// to turn that refusal off. The x/text decoders follow the WHATWG rule and
// substitute U+FFFD for input the encoding has no meaning for, so bytes that
// were never valid Shift-JIS loaded as replacement characters and the run exited
// 0 — the same silent corruption the UTF-8 check exists to stop, arriving
// through the door the flag opened.
//
// Which bytes to look at depends on the encoding, so there are two checks rather
// than one. Shift-JIS, EUC-JP, and ISO-2022-JP cannot represent U+FFFD at all,
// so one in the decoder's output can only have been substituted. UTF-16 can
// represent it, so its output says nothing and the source bytes are checked
// instead: a code unit is two bytes, and a surrogate means nothing alone.

// newDecodeValidatingReader wraps the decoded stream so a substitution fails the
// read, for the encodings where a U+FFFD in the output proves one. It returns
// decoded unchanged for the others, which are checked at the source instead.
func newDecodeValidatingReader(enc model.TextEncoding, decoded io.Reader) io.Reader {
	switch enc {
	case model.TextEncodingShiftJIS, model.TextEncodingEUCJP, model.TextEncodingISO2022JP:
		return &replacementDetectingReader{reader: decoded, encoding: enc}
	default:
		return decoded
	}
}

// newSourceValidatingReader wraps the undecoded stream for the encodings whose
// output cannot tell a substitution from data. It returns source unchanged for
// the others.
func newSourceValidatingReader(enc model.TextEncoding, source io.Reader) io.Reader {
	switch enc {
	case model.TextEncodingUTF16LE:
		return &utf16ValidatingReader{reader: source, encoding: enc, littleEndian: true}
	case model.TextEncodingUTF16BE:
		return &utf16ValidatingReader{reader: source, encoding: enc}
	default:
		return source
	}
}

// replacementDetectingReader fails the read on the first U+FFFD, which for an
// encoding that cannot write one is proof the decoder substituted it.
type replacementDetectingReader struct {
	reader   io.Reader
	encoding model.TextEncoding
	// pending holds the bytes of a rune split across two reads, so a replacement
	// character that straddles the boundary is still recognized.
	pending []byte
	// offset counts the decoded bytes seen, so the error can say where.
	offset int64
}

func (r *replacementDetectingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if invalid := r.scan(p[:n]); invalid != nil {
			return n, invalid
		}
	}
	return n, err
}

// scan walks one chunk, carrying over a rune the previous chunk ended inside.
func (r *replacementDetectingReader) scan(chunk []byte) error {
	buf := chunk
	if len(r.pending) > 0 {
		buf = append(r.pending, chunk...)
	}
	for len(buf) > 0 {
		if !utf8.FullRune(buf) {
			// The rest may still become a rune once more input arrives.
			r.pending = append(r.pending[:0], buf...)
			return nil
		}
		char, size := utf8.DecodeRune(buf)
		if char == utf8.RuneError && size == utf8.RuneLen(utf8.RuneError) {
			return substitutionError(r.encoding, r.offset)
		}
		buf = buf[size:]
		r.offset += int64(size)
	}
	r.pending = r.pending[:0]
	return nil
}

// utf16ValidatingReader fails the read on source bytes that are not UTF-16: an
// odd total length, or a surrogate with no partner.
type utf16ValidatingReader struct {
	reader       io.Reader
	encoding     model.TextEncoding
	littleEndian bool
	// half holds a byte left over when a read ended between the two bytes of a
	// code unit.
	half    []byte
	offset  int64
	pending bool // a high surrogate is waiting for its low half
}

func (r *utf16ValidatingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		if invalid := r.scan(p[:n]); invalid != nil {
			return n, invalid
		}
	}
	if errors.Is(err, io.EOF) {
		if len(r.half) > 0 {
			return n, fmt.Errorf("a %s code unit is two bytes, and the input ends after an odd number at offset %d", r.encoding, r.offset)
		}
		if r.pending {
			return n, fmt.Errorf("the %s input ends on an unpaired surrogate at offset %d", r.encoding, r.offset)
		}
	}
	return n, err
}

func (r *utf16ValidatingReader) scan(chunk []byte) error {
	buf := chunk
	if len(r.half) > 0 {
		buf = append(r.half, chunk...)
	}
	for len(buf) >= 2 {
		unit := uint16(buf[0])<<8 | uint16(buf[1])
		if r.littleEndian {
			unit = uint16(buf[1])<<8 | uint16(buf[0])
		}
		switch {
		case unit >= 0xD800 && unit <= 0xDBFF:
			if r.pending {
				return fmt.Errorf("the %s input has a high surrogate following another at offset %d", r.encoding, r.offset)
			}
			r.pending = true
		case unit >= 0xDC00 && unit <= 0xDFFF:
			if !r.pending {
				return fmt.Errorf("the %s input has a low surrogate with nothing before it at offset %d", r.encoding, r.offset)
			}
			r.pending = false
		default:
			if r.pending {
				return fmt.Errorf("the %s input has a high surrogate with nothing after it at offset %d", r.encoding, r.offset)
			}
		}
		buf = buf[2:]
		r.offset += 2
	}
	r.half = append(r.half[:0], buf...)
	return nil
}

// substitutionError words the refusal. It names the encoding the run declared,
// because that is the thing to change: the bytes are not wrong on their own,
// they are wrong for the encoding they were read as.
func substitutionError(enc model.TextEncoding, offset int64) error {
	return fmt.Errorf(
		"byte at offset %d is not valid %s, so it would be read as the replacement character; check %s, one of: %s",
		offset, enc, encodingFlag, model.TextEncodingHelp())
}
