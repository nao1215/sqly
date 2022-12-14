package infrastructure

import (
	"strings"

	"github.com/nao1215/sqly/domain/model"
)

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

func SingleQuote(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + len("''"))

	buf.WriteByte('\'')
	for _, r := range s {
		if r == '\'' {
			buf.WriteByte('\'')
		}
		buf.WriteRune(r)
	}
	buf.WriteByte('\'')
	return buf.String()
}

func GenerateCreateTableStatement(t *model.Table) string {
	ddl := "CREATE TABLE " + Quote(t.Name) + "("
	for i, v := range t.Header {
		ddl += Quote(v)
		if i != len(t.Header)-1 {
			ddl += ", "
		} else {
			ddl += ");"
		}
	}
	return ddl
}

func GenerateInsertStatement(name string, record model.Record) string {
	dml := "INSERT INTO " + Quote(name) + " VALUES ("
	for i, v := range record {
		dml += SingleQuote(v)
		if i != len(record)-1 {
			dml += ", "
		} else {
			dml += ");"
		}
	}
	return dml
}
