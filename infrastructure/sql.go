package infrastructure

import (
	"strings"
)

// Quote returns quoted string.
func Quote(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + len("``"))

	buf.WriteByte('`')
	for _, r := range s {
		if r == '`' {
			buf.WriteByte('`')
		}
		buf.WriteRune(r)
	}
	buf.WriteByte('`')
	return buf.String()
}

// QuoteTableRef quotes a possibly schema-qualified table reference. A bare name
// "user" becomes `user`; a qualified name "main.user" becomes `main`.`user`, so a
// helper command can reference the same schema-qualified table SQLite accepts in a
// query. The split happens only when the prefix is a real SQLite schema (main or
// temp); sqly rejects ATTACH/DETACH, so those are the only schemas a session can
// have. Any other dotted name (e.g. "a.b") is a single literal identifier and is
// quoted whole as `a.b`, matching `SELECT * FROM "a.b"`.
func QuoteTableRef(name string) string {
	if i := strings.IndexByte(name, '.'); i > 0 && i < len(name)-1 && isSchemaName(name[:i]) {
		return Quote(name[:i]) + "." + Quote(name[i+1:])
	}
	return Quote(name)
}

// isSchemaName reports whether prefix is a SQLite schema name a sqly session can
// reference: only "main" or "temp" (case-insensitive), since ATTACH/DETACH is
// rejected. A dotted prefix that is not one of these belongs to a literal table
// name rather than a schema qualifier.
func isSchemaName(prefix string) bool {
	switch strings.ToLower(prefix) {
	case "main", "temp":
		return true
	default:
		return false
	}
}
