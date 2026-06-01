package infrastructure

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

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
	indexTypeMap := make(map[int]string, len(t.Header()))
	semaphore := make(chan int, runtime.NumCPU())
	wg := &sync.WaitGroup{}

	var mu sync.RWMutex
	for i := range t.Header() {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			semaphore <- 1
			defer func() { <-semaphore }()
			if isNumeric(t, i) {
				mu.Lock()
				indexTypeMap[i] = "INTEGER"
				mu.Unlock()
			} else {
				mu.Lock()
				indexTypeMap[i] = "TEXT"
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	var builder strings.Builder
	builder.WriteString("CREATE TABLE " + Quote(t.Name()) + "(")
	for i, v := range t.Header() {
		fmt.Fprintf(&builder, "%s %s", Quote(v), indexTypeMap[i])
		if i != len(t.Header())-1 {
			builder.WriteString(", ")
		} else {
			builder.WriteString(");")
		}
	}
	return builder.String()
}

// isNumeric returns true if all records are numeric.
func isNumeric(t *model.Table, index int) bool {
	if len(t.Records()) == 0 {
		return false
	}

	for _, record := range t.Records() {
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
	var builder strings.Builder
	builder.WriteString("INSERT INTO " + Quote(name) + " VALUES (")
	for i, v := range record {
		builder.WriteString(SingleQuote(v))
		if i != len(record)-1 {
			builder.WriteString(", ")
		} else {
			builder.WriteString(");")
		}
	}
	return builder.String()
}
