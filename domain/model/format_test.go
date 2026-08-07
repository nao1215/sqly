package model

import (
	"strings"
	"testing"
)

// TestPrintModeRoundTrips is the property the registry exists to guarantee: a
// mode's name parses back to that mode. Before there was a registry, the mapping
// ran through two hand-written switches in two packages, and nothing checked
// they agreed — a format present in one and missing from the other was a
// compiling, passing, broken build.
func TestPrintModeRoundTrips(t *testing.T) {
	t.Parallel()

	for _, mode := range PrintModes() {
		name := mode.String()
		if name == unknownPrintModeName {
			t.Errorf("mode %d is in PrintModes but has no name", mode)
			continue
		}
		got, ok := ParsePrintMode(name)
		if !ok {
			t.Errorf("ParsePrintMode(%q) failed for a mode PrintModes lists", name)
			continue
		}
		if got != mode {
			t.Errorf("ParsePrintMode(%q) = %d, want %d", name, got, mode)
		}
	}
}

// TestPrintModesCoversEveryDeclaredMode fails when a PrintMode constant is added
// without a registry entry. The constants are consecutive from PrintModeTable,
// so the highest one declared is the count, and a mode missing from the registry
// shows up as one whose String is the unknown-mode name.
func TestPrintModesCoversEveryDeclaredMode(t *testing.T) {
	t.Parallel()

	// PrintModeVertical is the last constant declared; update this when a mode is
	// added after it.
	for mode := PrintModeTable; mode <= PrintModeVertical; mode++ {
		if mode.String() == unknownPrintModeName {
			t.Errorf("PrintMode %d is declared but has no registry entry, so no flag can name it", mode)
		}
	}
	if got, want := len(PrintModes()), int(PrintModeVertical)+1; got != want {
		t.Errorf("PrintModes() has %d entries, want %d (one per declared mode)", got, want)
	}
}

// TestParsePrintModeNormalizes checks the two things a user types that should not
// matter. --output-format took a value from a shell that may have padded it, and
// .mode is typed by hand; rejecting "CSV" taught nothing about what was wrong.
func TestParsePrintModeNormalizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  PrintMode
		ok    bool
	}{
		{"exact", "csv", PrintModeCSV, true},
		{"upper case", "CSV", PrintModeCSV, true},
		{"mixed case", "JsOnL", PrintModeJSONL, true},
		{"surrounding whitespace", "  markdown  ", PrintModeMarkdownTable, true},
		{"both", "\tVERTICAL\n", PrintModeVertical, true},
		{"not a format", "yaml", PrintModeTable, false},
		{"empty", "", PrintModeTable, false},
		{"a format's extension is not its name", ".csv", PrintModeTable, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParsePrintMode(tt.input)
			if ok != tt.ok {
				t.Fatalf("ParsePrintMode(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ParsePrintMode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// TestPrintModeNamesListsEveryMode locks the list a user is shown to the modes
// that exist, so a help line or an error cannot advertise a format that does not
// parse, or omit one that does.
func TestPrintModeNamesListsEveryMode(t *testing.T) {
	t.Parallel()

	listed := strings.Split(PrintModeNames(), ", ")
	if len(listed) != len(PrintModes()) {
		t.Fatalf("PrintModeNames() lists %d formats, want %d", len(listed), len(PrintModes()))
	}
	for i, mode := range PrintModes() {
		if listed[i] != mode.String() {
			t.Errorf("PrintModeNames()[%d] = %q, want %q (the order must match PrintModes)", i, listed[i], mode.String())
		}
	}
}

// TestSelectableModesAreTheOnesAScreenCanShow pins the two vocabularies apart.
//
// --output-format names a file format for --output, and .mode names what the
// screen shows. Excel and Parquet belong to the first only: neither can be
// rendered to a terminal, so .mode used to accept a name and then print CSV
// under it. The sets are asserted rather than derived from each other, so
// widening one does not silently widen the other.
func TestSelectableModesAreTheOnesAScreenCanShow(t *testing.T) {
	t.Parallel()

	wantSelectable := []string{"table", "vertical", "csv", "tsv", "ltsv", "json", "jsonl", "markdown"}
	wantOutputOnly := []string{"excel", "parquet"}

	got := strings.Split(SelectableModeNames(), ", ")
	if len(got) != len(wantSelectable) {
		t.Fatalf("SelectableModeNames() = %v, want %v", got, wantSelectable)
	}
	for i, want := range wantSelectable {
		if got[i] != want {
			t.Errorf("SelectableModeNames()[%d] = %q, want %q", i, got[i], want)
		}
	}

	// Every selectable name parses as one, and the file-only names do not.
	for _, name := range wantSelectable {
		if _, ok := ParseSelectableMode(name); !ok {
			t.Errorf("ParseSelectableMode(%q) = false, want true", name)
		}
	}
	for _, name := range wantOutputOnly {
		if _, ok := ParseSelectableMode(name); ok {
			t.Errorf("ParseSelectableMode(%q) = true, want false: it names a file, not a screen", name)
		}
		// --output-format still takes them, which is the point of keeping two lists.
		if _, ok := ParsePrintMode(name); !ok {
			t.Errorf("ParsePrintMode(%q) = false, want true: --output-format still names it", name)
		}
		if !strings.Contains(PrintModeNames(), name) {
			t.Errorf("PrintModeNames() lost %q, which --output-format accepts", name)
		}
	}

	// SelectableModes and SelectableModeNames must not disagree.
	if len(SelectableModes()) != len(got) {
		t.Errorf("SelectableModes() has %d entries, SelectableModeNames() lists %d", len(SelectableModes()), len(got))
	}
	for i, m := range SelectableModes() {
		if m.String() != got[i] {
			t.Errorf("SelectableModes()[%d] = %q, SelectableModeNames()[%d] = %q", i, m.String(), i, got[i])
		}
	}
}

// TestStdinFormatsHaveExtensions checks what the two tables this replaced could
// not: every name the flag accepts stages a file with an extension, so a value
// that validates cannot then fail to import for want of one.
func TestStdinFormatsHaveExtensions(t *testing.T) {
	t.Parallel()

	names := strings.Split(StdinFormatNames(), ", ")
	if len(names) == 0 {
		t.Fatal("StdinFormatNames() is empty")
	}
	for _, name := range names {
		ext, ok := StdinFormatExtension(name)
		if !ok {
			t.Errorf("StdinFormatNames() offers %q, which StdinFormatExtension does not know", name)
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			t.Errorf("StdinFormatExtension(%q) = %q, want a leading dot", name, ext)
		}
	}
}

// TestStdinFormatExtensionRejectsUnknown confirms an unknown format is refused
// rather than staged with an empty extension, which would produce an import
// failure that named neither the flag nor the format.
func TestStdinFormatExtensionRejectsUnknown(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "xlsx", "parquet", "CSV", " csv "} {
		if ext, ok := StdinFormatExtension(name); ok {
			t.Errorf("StdinFormatExtension(%q) = %q, true; want it refused", name, ext)
		}
	}
}
