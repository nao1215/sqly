package shell

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
	"testing/iotest"

	"github.com/nao1215/sqly/domain/model"
)

// readAllVerdict reads r to the end and reports what a caller sees: the bytes,
// and whether the read failed.
func readAllVerdict(r io.Reader) (string, bool) {
	got, err := io.ReadAll(r)
	return string(got), err != nil
}

// TestValidatingReadersDoNotDependOnHowTheInputIsSplit checks that the UTF-16
// and replacement-character checks reach the same verdict whatever size the
// reads come in. A code unit or a rune split between two reads is the case a
// file read in 32 KiB blocks meets at every block boundary.
func TestValidatingReadersDoNotDependOnHowTheInputIsSplit(t *testing.T) {
	t.Parallel()

	le := func(units ...uint16) []byte {
		b := make([]byte, 0, 2*len(units))
		for _, u := range units {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
		return b
	}
	be := func(units ...uint16) []byte {
		b := make([]byte, 0, 2*len(units))
		for _, u := range units {
			b = binary.BigEndian.AppendUint16(b, u)
		}
		return b
	}
	tests := []struct {
		name    string
		enc     model.TextEncoding
		source  bool // a source check (UTF-16) rather than a decoded-text check
		input   []byte
		wantErr bool
	}{
		{"utf-16le text", model.TextEncodingUTF16LE, true, le('a', 0x3042, 0xD83C, 0xDF63, '\n'), false},
		{"utf-16be text", model.TextEncodingUTF16BE, true, be('a', 0x3042, 0xD83C, 0xDF63, '\n'), false},
		{"utf-16le odd length", model.TextEncodingUTF16LE, true, append(le('a'), 'b'), true},
		{"utf-16le lone high surrogate", model.TextEncodingUTF16LE, true, le('a', 0xD83C, 'b'), true},
		{"utf-16le high surrogate at the end", model.TextEncodingUTF16LE, true, le('a', 0xD83C), true},
		{"utf-16le two high surrogates", model.TextEncodingUTF16LE, true, le(0xD83C, 0xD83C, 0xDF63), true},
		{"utf-16be lone low surrogate", model.TextEncodingUTF16BE, true, be('a', 0xDF63), true},
		{"decoded shift-jis text", model.TextEncodingShiftJIS, false, []byte("名前,🍣\n"), false},
		{"decoded replacement character", model.TextEncodingShiftJIS, false, []byte("名前,�\n"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wrap := func(r io.Reader) io.Reader {
				if tt.source {
					return newSourceValidatingReader(tt.enc, r)
				}
				return newDecodeValidatingReader(tt.enc, r)
			}
			whole, wholeErr := readAllVerdict(wrap(bytes.NewReader(tt.input)))
			if wholeErr != tt.wantErr {
				t.Fatalf("read whole: error = %v, want %v", wholeErr, tt.wantErr)
			}
			if !tt.wantErr && whole != string(tt.input) {
				t.Fatalf("read whole: the bytes changed")
			}
			for name, split := range map[string]func(io.Reader) io.Reader{
				"one byte": iotest.OneByteReader,
				"half":     iotest.HalfReader,
			} {
				got, gotErr := readAllVerdict(wrap(split(bytes.NewReader(tt.input))))
				if gotErr != tt.wantErr || (!tt.wantErr && got != whole) {
					t.Errorf("read %s at a time: (%q, error %v), want (%q, error %v)", name, got, gotErr, whole, tt.wantErr)
				}
			}
		})
	}
}
