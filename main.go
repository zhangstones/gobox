package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		usage(stdout)
		return 1
	}

	cmd := args[0]
	args = args[1:]

	switch cmd {
	case "find":
		if err := findCmd(args); err != nil {
			fmt.Fprintln(stderr, "find:", err)
			return 2
		}
	case "du":
		if err := duCmd(args); err != nil {
			fmt.Fprintln(stderr, "du:", err)
			return 2
		}
	case "ps":
		if err := psCmd(args); err != nil {
			fmt.Fprintln(stderr, "ps:", err)
			return 2
		}
	case "top":
		if err := topCmd(args); err != nil {
			fmt.Fprintln(stderr, "top:", err)
			return 2
		}
	case "iostat":
		if err := iostatCmd(args); err != nil {
			fmt.Fprintln(stderr, "iostat:", err)
			return 2
		}
	case "netstat":
		if err := netstatCmd(args); err != nil {
			fmt.Fprintln(stderr, "netstat:", err)
			return 2
		}
	case "xargs":
		if err := xargsCmd(args); err != nil {
			fmt.Fprintln(stderr, "xargs:", err)
			return 2
		}
	case "--help", "-h", "help":
		usage(stdout)
		return 0
	case "--version", "version", "-v":
		fmt.Fprintln(stdout, "gobox 0.1 - BusyBox-like toolset (partial)")
		return 0
	default:
		fmt.Fprintln(stderr, "unknown command:", cmd)
		usage(stdout)
		return 127
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "gobox - minimal BusyBox-like utility (partial)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: gobox <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  find     Search for files in a directory tree")
	fmt.Fprintln(w, "  du       Show file/directory disk usage")
	fmt.Fprintln(w, "  ps       List processes")
	fmt.Fprintln(w, "  top      Live process viewer")
	fmt.Fprintln(w, "  iostat   Show block device I/O stats (Linux cgroup/blkio)")
	fmt.Fprintln(w, "  netstat  Show network connection status")
	fmt.Fprintln(w, "  xargs    Build and execute command lines from stdin")
	fmt.Fprintln(w, "  version  Print program version (-v, --version)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags differ from BusyBox; this is a best-effort minimal implementation.")
}
