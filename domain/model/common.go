// Package model defines Data Transfer Object (Entity, Value Object)
package model

import (
	"fmt"
	"strings"
)

// Header is CSV/TSV/Table header.
type Header []string

// EnsureHeaderReimportable reports an error when a header holds two column names
// that an import would read as one column.
//
// Such a header is refused on the way in, so writing one reports success and
// leaves behind a file sqly cannot load. That is the silent loss a value the
// format cannot represent used to cause, and it is refused for the same reason.
//
// An import applies two comparisons, and either alone is a refusal, so both are
// checked here rather than the stricter single rule that combining them gives.
// Names are compared with surrounding whitespace removed, making "x" and " x"
// one name; and the CREATE TABLE behind the import compares them as they are
// with ASCII case ignored, making "a" and "A" one name. Neither rejects a pair
// that only the two together would, such as "a" beside " A", and that header
// loads.
//
// The fold stops at ASCII because that is the comparison SQLite makes: "ä" and
// "Ä" are two columns to it, and folding them would refuse a header that loads.
func EnsureHeaderReimportable(format string, header Header) error {
	trimmed := make(map[string]string, len(header))
	folded := make(map[string]string, len(header))
	for _, name := range header {
		if first, ok := trimmed[strings.TrimSpace(name)]; ok {
			return headerConflictError(format, first, name)
		}
		if first, ok := folded[asciiFold(name)]; ok {
			return headerConflictError(format, first, name)
		}
		trimmed[strings.TrimSpace(name)] = name
		folded[asciiFold(name)] = name
	}
	return nil
}

// headerConflictError words the refusal above. It names both columns because
// they are not always the same text: naming one is enough for a repeat, and says
// nothing at all when the pair differs only in case or in whitespace.
func headerConflictError(format, first, second string) error {
	return fmt.Errorf(
		"%s: column names %q and %q are one column to an import, which compares them with surrounding whitespace removed and ASCII case ignored, so the file could not be read back; alias one of them (SELECT a AS a1, b AS a2)",
		format, first, second)
}

// asciiFold lowercases the ASCII letters in s and leaves every other byte as it
// is, which is how SQLite compares two column names: its default case folding
// stops at ASCII, so "ä" and "Ä" stay two names. It returns s unchanged when
// there is nothing to fold, which is the common case.
func asciiFold(s string) string {
	var folded []byte
	for i := range len(s) {
		c := s[i]
		if c < 'A' || c > 'Z' {
			continue
		}
		if folded == nil {
			folded = []byte(s)
		}
		folded[i] = c + ('a' - 'A')
	}
	if folded == nil {
		return s
	}
	return string(folded)
}

// Equal compare Header.
func (h Header) Equal(h2 Header) bool {
	if len(h) != len(h2) {
		return false
	}
	for i, v := range h {
		if v != h2[i] {
			return false
		}
	}
	return true
}

// Record is CSV/TSV/Table records.
type Record []string

// Equal compare Record.
func (r Record) Equal(r2 Record) bool {
	if len(r) != len(r2) {
		return false
	}
	for i, v := range r {
		if v != r2[i] {
			return false
		}
	}
	return true
}
