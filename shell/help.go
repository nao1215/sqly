package shell

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"github.com/nao1215/sqly/config"
)

// helpLine is one helper command's usage and what it does, as shown by .help.
// usage carries the minimal argument shape (e.g. ".import PATH...") so the syntax
// of path-taking and destructive commands is visible without leaving the shell.
type helpLine struct {
	usage string
	desc  string
}

// helpGroup is a titled set of help lines, grouping commands by purpose so .help
// reads as a task list rather than a flat alphabetical dump.
type helpGroup struct {
	title string
	lines []helpLine
}

// helpGroups returns the grouped .help layout. The destructive in-place overwrite
// is listed on its own line so it is visually distinct from the non-destructive
// exports (.dump and .save DIR).
func helpGroups() []helpGroup {
	return []helpGroup{
		{"Session", []helpLine{
			{helpCommand, "show this help"},
			{modeCommand + " MODE", "change output mode (table, csv, tsv, ltsv, json, ndjson, markdown, ...)"},
			{clearCommand, "clear the terminal screen"},
			{exitCommand, "exit sqly"},
		}},
		{"Navigate", []helpLine{
			{cdCommand + " [DIR]", "change the working directory (no arg: home)"},
			{pwdCommand, "print the working directory"},
			{lsCommand + " [DIR]", "list directory contents"},
		}},
		{"Inspect", []helpLine{
			{tablesCommand, "list imported tables"},
			{schemaCommand + " TABLE", "print a table's CREATE statement"},
			{describeCommand + " TABLE", "print a table's columns"},
			{headerCommand + " TABLE", "print a table's header row"},
		}},
		{"Import / Export", []helpLine{
			{importCommand + " PATH...", "load files or directories into the session"},
			{importModeCommand + " POLICY", "set ragged CSV/TSV row handling (stop, skip, fill)"},
			{dumpCommand + " TABLE FILE", "export a table to a file (format follows .mode; default csv)"},
			{saveCommand + " DIR", "write changed tables into DIR (sources untouched)"},
			{saveCommand + " --force", "overwrite each table's source file in place (destructive)"},
		}},
	}
}

// helpCommand prints the helper commands grouped by purpose, with a minimal usage
// suffix for each so common tasks and risky operations are discoverable in-shell.
func (c CommandList) helpCommand(_ context.Context, _ *Shell, argv []string) error {
	if len(argv) > 0 {
		return fmt.Errorf(".help takes no arguments, got %d", len(argv))
	}
	fmt.Fprintln(config.Stdout, "sqly helper commands (run inside the shell; SQL needs no leading dot):")
	for _, g := range helpGroups() {
		fmt.Fprintf(config.Stdout, "\n%s\n", g.title)
		for _, ln := range g.lines {
			// Pad before coloring so the column stays aligned when color is disabled
			// (tests, pipes); the trailing spaces sit inside the colored span.
			fmt.Fprintf(config.Stdout, "  %s %s\n", color.CyanString("%-18s", ln.usage), ln.desc)
		}
	}
	return nil
}
