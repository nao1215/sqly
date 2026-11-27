// Package sqlname reads the table references a sqly session can write.
package sqlname

import "strings"

// SplitQualifier splits a possibly schema-qualified table reference such as
// "main.user" or "temp.t" into its schema and object name. A bare name yields an
// empty schema. The split happens only when the prefix before the first dot is a
// schema a session can have, main or temp in any case, since sqly rejects ATTACH
// and DETACH, and something follows the dot. Any other dotted name, such as
// "a.b", is one literal table identifier and is returned whole, matching
// SELECT * FROM "a.b".
func SplitQualifier(name string) (schema, object string) {
	if prefix, rest, found := strings.Cut(name, "."); found && rest != "" && isSchema(prefix) {
		return prefix, rest
	}
	return "", name
}

func isSchema(prefix string) bool {
	switch strings.ToLower(prefix) {
	case "main", "temp":
		return true
	default:
		return false
	}
}
