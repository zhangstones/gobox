package main

import (
	"fmt"
	"io"
	"os"

	"gobox/cmds/disk"
	"gobox/cmds/fs"
	"gobox/cmds/net"
	"gobox/cmds/proc"
	"gobox/cmds/text"
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

	var err error
	switch cmd {
	case "find":
		err = fs.FindCmd(args)
	case "du":
		err = fs.DuCmd(args)
	case "ps":
		err = proc.PsCmd(args)
	case "top":
		err = proc.TopCmd(args)
	case "iostat":
		err = disk.IostatCmd(args)
	case "netstat":
		err = net.NetstatCmd(args)
	case "xargs":
		err = proc.XargsCmd(args)
	case "grep":
		err = text.GrepCmd(args)
	case "sed":
		err = text.SedCmd(args)
	case "dig", "nslookup":
		err = net.DigCmd(args)
	case "sort":
		err = text.SortCmd(args)
	case "head":
		err = text.HeadCmd(args)
	case "tail":
		err = text.TailCmd(args)
	case "curl":
		err = net.CurlCmd(args)
	case "wc":
		err = text.WcCmd(args)
	case "uniq":
		err = text.UniqCmd(args)
	case "nc":
		err = net.NcCmd(args)
	case "tw":
		err = net.TwCmd(args)
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

	if err != nil {
		fmt.Fprintln(stderr, cmd+":", err)
		return 2
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
