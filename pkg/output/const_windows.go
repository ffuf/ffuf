//go:build windows
// +build windows

package output

const (
	TERMINAL_CLEAR_LINE        = "\r\r"
	BEFORE_TERMINAL_CLEAR_LINE = "\x1b[1K \x1b[100D"
	ANSI_CLEAR                 = ""
	ANSI_RED                   = ""
	ANSI_GREEN                 = ""
	ANSI_BLUE                  = ""
	ANSI_YELLOW                = ""
)
