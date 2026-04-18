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
	case "grep":
		if err := grepCmd(args); err != nil {
			fmt.Fprintln(stderr, "grep:", err)
			return 2
		}
	case "sed":
		if err := sedCmd(args); err != nil {
			fmt.Fprintln(stderr, "sed:", err)
			return 2
		}
	case "dig":
		if err := digCmd(args); err != nil {
			fmt.Fprintln(stderr, "dig:", err)
			return 2
		}
	case "sort":
		if err := sortCmd(args); err != nil {
			fmt.Fprintln(stderr, "sort:", err)
			return 2
		}
	case "head":
		if err := headCmd(args); err != nil {
			fmt.Fprintln(stderr, "head:", err)
			return 2
		}
	case "tail":
		if err := tailCmd(args); err != nil {
			fmt.Fprintln(stderr, "tail:", err)
			return 2
		}
	case "curl":
		if err := curlCmd(args); err != nil {
			fmt.Fprintln(stderr, "curl:", err)
			return 2
		}
	case "wc":
		if err := wcCmd(args); err != nil {
			fmt.Fprintln(stderr, "wc:", err)
			return 2
		}
	case "uniq":
		if err := uniqCmd(args); err != nil {
			fmt.Fprintln(stderr, "uniq:", err)
			return 2
		}
	case "nc":
		if err := ncCmd(args); err != nil {
			fmt.Fprintln(stderr, "nc:", err)
			return 2
		}
	case "tw":
		if err := twCmd(args); err != nil {
			fmt.Fprintln(stderr, "tw:", err)
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
	fmt.Fprintln(w, "  grep     Search for patterns in files (regex support)")
	fmt.Fprintln(w, "  sed      Stream editor for filtering and transforming text")
	fmt.Fprintln(w, "  dig      DNS lookup utility")
	fmt.Fprintln(w, "  head     Print the first lines of a file")
	fmt.Fprintln(w, "  tail     Print the last lines of a file")
	fmt.Fprintln(w, "  uniq     Filter adjacent matching lines")
	fmt.Fprintln(w, "  curl     Transfer data from a URL")
	fmt.Fprintln(w, "  wc       Print line, word, and byte counts")
	fmt.Fprintln(w, "  nc       Netcat - arbitrary TCP/UDP connections and listening")
	fmt.Fprintln(w, "  tw       Tiny web server for static files or benchmark")
	fmt.Fprintln(w, "  version  Print program version (-v, --version)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags differ from BusyBox; this is a best-effort minimal implementation.")
}
