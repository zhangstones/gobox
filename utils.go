package main

import (
	"os"
)

// isStdoutTerminal returns true if stdout is a terminal. This implementation
// avoids external dependencies by using os.FileInfo mode checks. It's not as
// precise as platform-specific ioctl checks, but sufficient for distinguishing
// between a terminal and pipe/redirect in most environments.
func isStdoutTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// If stdout is a character device, assume it's a terminal. If it's a pipe or
	// regular file (redirect), this will be false.
	return (fi.Mode() & os.ModeCharDevice) != 0
}
