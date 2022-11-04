package shell

import "strings"

func trimWordGaps(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
