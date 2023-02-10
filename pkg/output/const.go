//go:build !windows
// +build !windows

package output

const (
	TERMINAL_CLEAR_LINE        = "\r\x1b[2K"
	BEFORE_TERMINAL_CLEAR_LINE = ""
	ANSI_CLEAR                 = "\x1b[0m"
	ANSI_RED                   = "\x1b[31m"
	ANSI_GREEN                 = "\x1b[32m"
	ANSI_BLUE                  = "\x1b[34m"
	ANSI_YELLOW                = "\x1b[33m"
)
