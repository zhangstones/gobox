package utils

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// IsTerminal returns true if the given writer is a terminal.
func IsTerminal(w io.Writer) bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// StdoutWidth returns the current stdout terminal width when available.
func StdoutWidth() (int, bool) {
	width, _, ok := StdoutSize()
	return width, ok
}

// StdoutHeight returns the current stdout terminal height when available.
func StdoutHeight() (int, bool) {
	_, height, ok := StdoutSize()
	return height, ok
}

// StdoutSize returns the current stdout terminal width and height when available.
func StdoutSize() (int, int, bool) {
	width := 0
	height := 0
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			width = n
		}
	}
	if lines := os.Getenv("LINES"); lines != "" {
		if n, err := strconv.Atoi(lines); err == nil && n > 0 {
			height = n
		}
	}
	if width > 0 && height > 0 {
		return width, height, true
	}
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		if width > 0 || height > 0 {
			return width, height, true
		}
		return 0, 0, false
	}
	if width == 0 && ws.Col > 0 {
		width = int(ws.Col)
	}
	if height == 0 && ws.Row > 0 {
		height = int(ws.Row)
	}
	if width == 0 && height == 0 {
		return 0, 0, false
	}
	return width, height, true
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
