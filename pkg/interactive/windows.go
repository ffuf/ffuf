//go:build windows
// +build windows

package interactive

import (
	"os"
	"syscall"
)

func termHandle() (*os.File, error) {
	var tty *os.File
	_, err := syscall.Open("CONIN$", syscall.O_RDWR, 0)
	if err != nil {
		return tty, err
	}
	tty, err = os.Open("CONIN$")
	if err != nil {
		return tty, err
	}
	return tty, nil
}
