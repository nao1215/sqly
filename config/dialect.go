package config

import (
	"strings"

	"github.com/nao1215/filesql/dialect"
)

// DialectNames returns the SQL dialect names sqly accepts, in the order filesql
// declares them.
//
// Every list a user reads — the --dialect flag's help and error, the .dialect
// message, and shell completion — is built from this one call. Written out by
// hand, a dialect added upstream reaches only the lists someone remembers.
func DialectNames() []string {
	dialects := dialect.Dialects()
	names := make([]string, 0, len(dialects))
	for _, d := range dialects {
		names = append(names, string(d))
	}
	return names
}

// DialectNameList joins DialectNames for a message: "sqlite, mysql, postgresql,
// googlesql".
func DialectNameList() string {
	return strings.Join(DialectNames(), ", ")
}
