//go:build windows
// +build windows

package output

import (
	"os"

	"golang.org/x/sys/windows"
)

// terminalWidth returns the width of the terminal attached to stderr in
// columns, or 0 if it cannot be determined (e.g. stderr is not a console).
func terminalWidth() int {
	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(windows.Handle(os.Stderr.Fd()), &info); err != nil {
		return 0
	}
	return int(info.Window.Right - info.Window.Left + 1)
}
