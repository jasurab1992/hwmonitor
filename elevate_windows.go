//go:build windows

package main

import (
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const alreadyElevatedArg = "--already-elevated"

// isElevated returns true if the current process has administrator privileges.
func isElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// maybeElevate checks if elevation is needed and re-launches via UAC if so.
// Returns true if the caller should exit (elevation was triggered or already tried).
func maybeElevate() bool {
	// If we already tried to elevate, don't loop.
	for _, a := range os.Args[1:] {
		if a == alreadyElevatedArg {
			// Strip the flag so it doesn't confuse flag.Parse later.
			filtered := []string{os.Args[0]}
			for _, x := range os.Args[1:] {
				if x != alreadyElevatedArg {
					filtered = append(filtered, x)
				}
			}
			os.Args = filtered
			return false
		}
	}

	if isElevated() {
		return false
	}

	// Not elevated and first attempt — relaunch with UAC.
	relaunchElevated()
	return true
}

// relaunchElevated re-launches the current executable with the "runas" verb
// via ShellExecuteW, which triggers a UAC prompt for elevation.
func relaunchElevated() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// Append marker so the elevated copy knows not to loop.
	args := strings.Join(append(os.Args[1:], alreadyElevatedArg), " ")

	verb, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argsPtr, _ := syscall.UTF16PtrFromString(args)

	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")
	shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(argsPtr)),
		0,
		1, // SW_SHOWNORMAL
	)
}
