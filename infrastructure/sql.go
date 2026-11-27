package infrastructure

import (
	"strings"

	"github.com/nao1215/sqly/internal/sqlname"
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
// query. What counts as a qualifier is sqlname.SplitQualifier's rule, so any other
// dotted name (e.g. "a.b") is quoted whole as `a.b`, matching `SELECT * FROM "a.b"`.
func QuoteTableRef(name string) string {
	if schema, object := sqlname.SplitQualifier(name); schema != "" {
		return Quote(schema) + "." + Quote(object)
	}
	return Quote(name)
}
