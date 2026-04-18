package utils

import (
	"fmt"
	"io"
	"os"
)

// IsTerminal returns true if the given writer is a terminal.
func IsTerminal(w io.Writer) bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// HumanSize formats bytes into human-readable string (KB, MB, GB, etc.)
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(b) / float64(div)
	suf := "KMGTPE"[exp]
	return fmt.Sprintf("%.1f%cB", value, suf)
}
