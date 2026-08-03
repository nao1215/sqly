package model

import (
	"fmt"
	"strings"
)

// TextEncoding selects how a text import without a Unicode BOM is decoded
// before parsing. It applies to CSV, TSV, LTSV, JSON, and JSONL inputs.
type TextEncoding string

const (
	// TextEncodingUTF8 selects UTF-8 input.
	TextEncodingUTF8 TextEncoding = "utf-8"
	// TextEncodingShiftJIS selects Shift-JIS input.
	TextEncodingShiftJIS TextEncoding = "shift-jis"
	// TextEncodingEUCJP selects EUC-JP input.
	TextEncodingEUCJP TextEncoding = "euc-jp"
	// TextEncodingISO2022JP selects ISO-2022-JP input.
	TextEncodingISO2022JP TextEncoding = "iso-2022-jp"
	// TextEncodingUTF16LE selects little-endian UTF-16 input.
	TextEncodingUTF16LE TextEncoding = "utf-16le"
	// TextEncodingUTF16BE selects big-endian UTF-16 input.
	TextEncodingUTF16BE TextEncoding = "utf-16be"
)

const textEncodingHelp = "utf-8|shift-jis|euc-jp|iso-2022-jp|utf-16le|utf-16be"

// TextEncodingHelp returns the user-facing list shared by --encoding and
// .encoding diagnostics.
func TextEncodingHelp() string { return textEncodingHelp }

// String returns the canonical encoding name used by flags and shell commands.
func (e TextEncoding) String() string {
	switch e {
	case TextEncodingUTF8,
		TextEncodingShiftJIS,
		TextEncodingEUCJP,
		TextEncodingISO2022JP,
		TextEncodingUTF16LE,
		TextEncodingUTF16BE:
		return string(e)
	default:
		return string(TextEncodingUTF8)
	}
}

// ParseTextEncoding converts a user-facing encoding name into its canonical
// form. Common aliases are accepted so flags and shell commands stay ergonomic.
func ParseTextEncoding(name string) (TextEncoding, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case string(TextEncodingUTF8), "utf8":
		return TextEncodingUTF8, nil
	case string(TextEncodingShiftJIS), "shift_jis", "shiftjis", "sjis", "cp932", "ms932", "windows-31j", "windows31j":
		return TextEncodingShiftJIS, nil
	case string(TextEncodingEUCJP), "eucjp":
		return TextEncodingEUCJP, nil
	case string(TextEncodingISO2022JP), "iso2022jp", "jis":
		return TextEncodingISO2022JP, nil
	case string(TextEncodingUTF16LE), "utf16le":
		return TextEncodingUTF16LE, nil
	case string(TextEncodingUTF16BE), "utf16be":
		return TextEncodingUTF16BE, nil
	default:
		return TextEncodingUTF8, fmt.Errorf("invalid text encoding %q: want %s", name, textEncodingHelp)
	}
}
