//go:build windows

package shell

import "errors"

// makeFIFO reports that Windows has no named pipe this test can create in the
// file system, so the caller skips.
func makeFIFO(string) error {
	return errors.New("named pipes are not created with mkfifo on windows")
}
