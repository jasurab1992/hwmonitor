//go:build windows

package ui

import (
	"os"

	"golang.org/x/sys/windows"
)

func init() {
	// Enable ANSI/VT100 escape sequence processing in Windows Console.
	// Required on Windows Server where it's not enabled by default.
	// Without this, ESC characters appear as squares instead of colors.
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(stdout, &mode); err != nil {
		return
	}
	windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
