//go:build !windows
// +build !windows

package output

import (
	"os"

	"golang.org/x/sys/unix"
)

// terminalWidth returns the width of the terminal attached to stderr in
// columns, or 0 if it cannot be determined (e.g. stderr is not a tty).
func terminalWidth() int {
	ws, err := unix.IoctlGetWinsize(int(os.Stderr.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0
	}
	return int(ws.Col)
}
