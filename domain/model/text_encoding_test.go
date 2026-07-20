package model

import "testing"

func TestTextEncoding_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		enc  TextEncoding
		want string
	}{
		{name: "utf-8", enc: TextEncodingUTF8, want: "utf-8"},
		{name: "shift-jis", enc: TextEncodingShiftJIS, want: "shift-jis"},
		{name: "euc-jp", enc: TextEncodingEUCJP, want: "euc-jp"},
		{name: "iso-2022-jp", enc: TextEncodingISO2022JP, want: "iso-2022-jp"},
		{name: "utf-16le", enc: TextEncodingUTF16LE, want: "utf-16le"},
		{name: "utf-16be", enc: TextEncodingUTF16BE, want: "utf-16be"},
		{name: "unknown falls back to utf-8", enc: TextEncoding("bogus"), want: "utf-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.enc.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseTextEncoding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    TextEncoding
		wantErr bool
	}{
		{name: "utf-8", input: "utf-8", want: TextEncodingUTF8},
		{name: "utf8 alias", input: "utf8", want: TextEncodingUTF8},
		{name: "shift-jis", input: "shift-jis", want: TextEncodingShiftJIS},
		{name: "cp932 alias", input: "cp932", want: TextEncodingShiftJIS},
		{name: "windows alias", input: "windows-31j", want: TextEncodingShiftJIS},
		{name: "euc-jp", input: "euc-jp", want: TextEncodingEUCJP},
		{name: "iso-2022-jp", input: "iso-2022-jp", want: TextEncodingISO2022JP},
		{name: "utf-16le", input: "utf-16le", want: TextEncodingUTF16LE},
		{name: "utf-16be", input: "utf-16be", want: TextEncodingUTF16BE},
		{name: "invalid", input: "latin1", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseTextEncoding(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTextEncoding(%q) expected an error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTextEncoding(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTextEncoding(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
