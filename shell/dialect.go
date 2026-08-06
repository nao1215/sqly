package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/filesql/dialect"
	"github.com/nao1215/sqly/config"
)

// dialectCommand shows or changes the SQL dialect applied to user queries.
//
//	.dialect            print the current dialect and the available choices
//	.dialect NAME       switch the dialect (sqlite, mysql, postgresql, googlesql)
//
// Loading always uses SQLite; only queries typed at the prompt (or run via
// --sql/--sql-file/batch) are translated.
func (c CommandList) dialectCommand(_ context.Context, s *Shell, argv []string) error {
	if len(argv) == 0 {
		printSessionSetting(settingDialect, string(s.usecases.query.Dialect()), config.DialectNameList())
		return nil
	}
	if len(argv) > 1 {
		return &invocationError{Err: fmt.Errorf(".dialect accepts a single dialect name, got %d arguments", len(argv))}
	}

	d, err := dialect.Parse(argv[0])
	if err != nil {
		return &invocationError{Err: fmt.Errorf("unknown SQL dialect %q (available: %s)", argv[0], config.DialectNameList())}
	}
	s.usecases.query.SetDialect(d)
	fmt.Fprintf(config.Stderr, "dialect set to %s\n", d)
	// Say what the choice means, at the moment it is made rather than at the
	// moment it bites. The switch itself is the answer to "why did my query
	// behave like SQLite?", and by the time a result looks wrong the connection
	// between the two is gone.
	s.warnDialectTranslationOnce(d)
	return nil
}

// warnDialectTranslationOnce tells the user, on stderr and at most once per
// session, that a non-SQLite dialect is translated rather than emulated.
//
// Why it exists: choosing --dialect postgresql looks like choosing PostgreSQL,
// and it is not. The syntax is rewritten; the engine underneath is SQLite, with
// SQLite's types, collation, NULL handling, and functions. A query that SQLite
// accepts runs, and a query whose meaning differs between the two engines
// returns a different answer with nothing to say so. That silence is the danger,
// and one line of stderr ends it.
//
// Why once: the fact is about the session, not about the statement. Repeating it
// per statement would make a script's stderr unreadable and would train the
// reader to ignore it, which is the same as not printing it.
//
// Why stderr: stdout carries results a program parses. Nothing here may reach
// it.
//
// SQLite is silent because there is nothing to translate: it is the identity
// translation and the engine's own dialect.
func (s *Shell) warnDialectTranslationOnce(d dialect.Dialect) {
	if s.dialectWarned || d == dialect.SQLite {
		return
	}
	s.dialectWarned = true
	// The spelling comes from filesql, which owns the dialect: a dialect added
	// there arrives already knowing how its own project writes its name.
	name := d.DisplayName()
	fmt.Fprintf(config.Stderr,
		"Warning: %s syntax is translated to SQLite; execution uses SQLite semantics, not %s semantics.\n",
		name, name)
}
