package infrastructure

import (
	"strconv"
	"strings"

	"github.com/nao1215/sqly/domain/model"
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

// SingleQuote returns single quoted string.
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

// GenerateCreateTableStatement returns create table statement.
// e.g. CREATE TABLE `table_name` (`column1` INTEGER, `column2` TEXT, ...);
func GenerateCreateTableStatement(t *model.Table) string {
	ddl := "CREATE TABLE " + Quote(t.Name) + "("
	for i, v := range t.Header {
		if isNumeric(t, i) {
			ddl += Quote(v) + " INTEGER"
		} else {
			ddl += Quote(v) + " TEXT"
		}
		if i != len(t.Header)-1 {
			ddl += ", "
		} else {
			ddl += ");"
		}
	}
	return ddl
}

// isNumeric returns true if all records are numeric.
func isNumeric(t *model.Table, index int) bool {
	if len(t.Records) == 0 {
		return false
	}

	for _, record := range t.Records {
		_, err := strconv.ParseFloat(record[index], 64)
		if err != nil {
			return false
		}
	}
	return true
}

// GenerateInsertStatement returns insert statement.
// e.g. INSERT INTO `table_name` VALUES ('value1', 'value2', ...);
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
