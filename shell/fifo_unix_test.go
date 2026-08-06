//go:build !windows

package shell

import "golang.org/x/sys/unix"

// makeFIFO creates a named pipe for the destination tests. Windows has no
// mkfifo, so the test that uses it is skipped there.
func makeFIFO(path string) error {
	return unix.Mkfifo(path, 0o600)
}
