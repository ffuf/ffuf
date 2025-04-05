//go:build !windows
// +build !windows

package interactive

import "os"

func termHandle() (*os.File, error) {
	return os.Open("/dev/tty")
}
