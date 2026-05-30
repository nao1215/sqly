package shell

import (
	"context"
	"fmt"

	"github.com/nao1215/sqly/config"
)

// clearCommand clears the terminal screen in-process.
//
// It writes ANSI escapes (clear screen, clear scrollback, cursor home) instead
// of shelling out to clear/cls. Why: spawning an external process can stall in
// headless CI and ties behavior to platform binaries. config.Stdout is a
// go-colorable writer, which translates these escapes to the Windows console
// API, so a single sequence works across supported operating systems.
func (c CommandList) clearCommand(_ context.Context, _ *Shell, _ []string) error {
	fmt.Fprint(config.Stdout, "\x1b[H\x1b[2J\x1b[3J")
	return nil
}
