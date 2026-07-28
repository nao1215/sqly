package shell

import (
	"context"
	"fmt"
	"strings"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/config"
)

// dialectCommand shows or changes the SQL dialect applied to user queries.
//
//	.dialect            print the current dialect and the available choices
//	.dialect NAME       switch the dialect (sqlite|mysql|postgresql|googlesql)
//
// Loading always uses SQLite; only queries typed at the prompt (or run via
// --sql/--sql-file/batch) are translated.
func (c CommandList) dialectCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		fmt.Fprintf(config.Stdout, "current dialect: %s (available: %s)\n",
			s.usecases.query.Dialect(), strings.Join(dialectNames(), ", "))
		return nil
	}
	if len(argv) > 1 {
		return fmt.Errorf(".dialect accepts a single dialect name, got %d arguments", len(argv))
	}

	d, err := dialect.Parse(argv[0])
	if err != nil {
		return fmt.Errorf("unknown SQL dialect %q (available: %s)", argv[0], strings.Join(dialectNames(), ", "))
	}
	s.usecases.query.SetDialect(d)
	fmt.Fprintf(config.Stdout, "dialect set to %s\n", d)
	return nil
}

// dialectNames returns the built-in dialect names in a stable order for help and
// error messages.
func dialectNames() []string {
	ds := dialect.Dialects()
	names := make([]string, 0, len(ds))
	for _, d := range ds {
		names = append(names, string(d))
	}
	return names
}
