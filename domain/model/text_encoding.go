package model

import (
	"fmt"
	"strings"
)

// TextEncoding selects how a text import without a Unicode BOM is decoded
// before parsing. It applies to CSV, TSV, LTSV, JSON, and JSONL inputs.
type TextEncoding string

const (
	TextEncodingUTF8      TextEncoding = "utf-8"
	TextEncodingShiftJIS  TextEncoding = "shift-jis"
	TextEncodingEUCJP     TextEncoding = "euc-jp"
	TextEncodingISO2022JP TextEncoding = "iso-2022-jp"
	TextEncodingUTF16LE   TextEncoding = "utf-16le"
	TextEncodingUTF16BE   TextEncoding = "utf-16be"
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
	case "utf-8", "utf8":
		return TextEncodingUTF8, nil
	case "shift-jis", "shift_jis", "shiftjis", "sjis", "cp932", "ms932", "windows-31j", "windows31j":
		return TextEncodingShiftJIS, nil
	case "euc-jp", "eucjp":
		return TextEncodingEUCJP, nil
	case "iso-2022-jp", "iso2022jp", "jis":
		return TextEncodingISO2022JP, nil
	case "utf-16le", "utf16le":
		return TextEncodingUTF16LE, nil
	case "utf-16be", "utf16be":
		return TextEncodingUTF16BE, nil
	default:
		return TextEncodingUTF8, fmt.Errorf("invalid text encoding %q: want %s", name, textEncodingHelp)
	}
}
