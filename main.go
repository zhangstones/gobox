package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "find":
		if err := findCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "find:", err)
			os.Exit(2)
		}
	case "du":
		if err := duCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "du:", err)
			os.Exit(2)
		}
	case "ps":
		if err := psCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "ps:", err)
			os.Exit(2)
		}
	case "top":
		if err := topCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "top:", err)
			os.Exit(2)
		}
	case "iostat":
		if err := iostatCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "iostat:", err)
			os.Exit(2)
		}
	case "netstat":
		if err := netstatCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "netstat:", err)
			os.Exit(2)
		}
	case "xargs":
		if err := xargsCmd(args); err != nil {
			fmt.Fprintln(os.Stderr, "xargs:", err)
			os.Exit(2)
		}
	case "--help", "-h", "help":
		usage()
	case "--version", "version", "-v":
		fmt.Println("gobox 0.1 - BusyBox-like toolset (partial)")
	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		usage()
		os.Exit(127)
	}
}

func usage() {
	fmt.Println("gobox - minimal BusyBox-like utility (partial)")
	fmt.Println()
	fmt.Println("Usage: gobox <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  find     Search for files in a directory tree")
	fmt.Println("  du       Show file/directory disk usage")
	fmt.Println("  ps       List processes")
	fmt.Println("  top      Live process viewer")
	fmt.Println("  iostat   Show block device I/O stats (Linux cgroup/blkio)")
	fmt.Println("  netstat  Show network conn status")
	fmt.Println("  xargs    Build and execute command lines from stdin")
	fmt.Println("  version  Print program version (-v, --version)")
	fmt.Println()
	fmt.Println("Flags differ from BusyBox; this is a best-effort minimal implementation.")
}
