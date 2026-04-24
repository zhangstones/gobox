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

type cliErrorSilencer interface {
	SuppressCLIError() bool
}

type cliExitCoder interface {
	ExitCode() int
}

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
	case "df":
		err = fs.DfCmd(args)
	case "readpath":
		err = fs.ReadpathCmd(args)
	case "stat":
		err = fs.StatCmd(args)
	case "truncate":
		err = fs.TruncateCmd(args)
	case "ps":
		err = proc.PsCmd(args)
	case "top":
		err = proc.TopCmd(args)
	case "free":
		err = proc.FreeCmd(args)
	case "iostat":
		err = disk.IostatCmd(args)
	case "ioperf":
		err = disk.IoperfCmd(args)
	case "md5sum":
		err = disk.Md5sumCmd(args)
	case "sha256sum":
		err = disk.Sha256sumCmd(args)
	case "netstat":
		err = net.NetstatCmd(args)
	case "ip":
		err = net.IpCmd(args)
	case "xargs":
		err = proc.XargsCmd(args)
	case "kill":
		err = proc.KillCmd(args)
	case "lsof":
		err = proc.LsofCmd(args)
	case "watch":
		err = proc.WatchCmd(args)
	case "timeout":
		err = proc.TimeoutCmd(args)
	case "grep":
		err = text.GrepCmd(args)
	case "sed":
		err = text.SedCmd(args)
	case "dig", "nslookup":
		err = net.DigCmd(args)
	case "sort":
		err = text.SortCmd(args)
	case "rand":
		err = text.RandCmd(args)
	case "head":
		err = text.HeadCmd(args)
	case "tail":
		err = text.TailCmd(args)
	case "curl":
		err = net.CurlCmd(args)
	case "wc":
		err = text.WcCmd(args)
	case "hex":
		err = text.HexCmd(args)
	case "base64":
		err = text.Base64Cmd(args)
	case "strings":
		err = text.StringsCmd(args)
	case "diff":
		err = text.DiffCmd(args)
	case "uniq":
		err = text.UniqCmd(args)
	case "nc":
		err = net.NcCmd(args)
	case "tw":
		err = net.TwCmd(args)
	case "ifstat":
		err = net.IfstatCmd(args)
	case "np":
		err = net.NpCmd(args)
	case "seq":
		err = text.SeqCmd(args)
	case "--help", "-h", "help":
		usage(stdout)
		return 0
	case "--version", "version", "-v":
		fmt.Fprintln(stdout, "gobox 0.1 - container troubleshooting toolset")
		return 0
	default:
		fmt.Fprintln(stderr, "unknown command:", cmd)
		usage(stdout)
		return 127
	}

	if err != nil {
		if exitErr, ok := err.(cliExitCoder); ok {
			if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
				fmt.Fprintln(stderr, cmd+":", err)
			}
			return exitErr.ExitCode()
		}
		if silencer, ok := err.(cliErrorSilencer); !ok || !silencer.SuppressCLIError() {
			fmt.Fprintln(stderr, cmd+":", err)
		}
		return 2
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "gobox - minimal container troubleshooting utility set")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage: gobox <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  find     Search for files in a directory tree")
	fmt.Fprintln(w, "  du       Show file/directory disk usage")
	fmt.Fprintln(w, "  df       Show filesystem usage")
	fmt.Fprintln(w, "  readpath Resolve paths and symlinks")
	fmt.Fprintln(w, "  stat     Show file or filesystem status")
	fmt.Fprintln(w, "  truncate Shrink or extend file size")
	fmt.Fprintln(w, "  ps       List processes")
	fmt.Fprintln(w, "  top      Live process viewer")
	fmt.Fprintln(w, "  free     Show memory usage")
	fmt.Fprintln(w, "  iostat   Show block device I/O stats (Linux cgroup/blkio)")
	fmt.Fprintln(w, "  ioperf   I/O performance benchmark tool (simplified fio-like)")
	fmt.Fprintln(w, "  md5sum   Compute/check MD5 checksums")
	fmt.Fprintln(w, "  sha256sum Compute/check SHA-256 checksums")
	fmt.Fprintln(w, "  netstat  Show network connection status")
	fmt.Fprintln(w, "  ip       Show network interfaces, routes, and neighbours")
	fmt.Fprintln(w, "  xargs    Build and execute command lines from stdin")
	fmt.Fprintln(w, "  kill     Send signals to processes")
	fmt.Fprintln(w, "  lsof     List open files")
	fmt.Fprintln(w, "  watch    Run a command periodically")
	fmt.Fprintln(w, "  timeout  Run a command with a time limit")
	fmt.Fprintln(w, "  grep     Search for patterns in files (regex support)")
	fmt.Fprintln(w, "  sed      Stream editor for filtering and transforming text")
	fmt.Fprintln(w, "  sort     Sort lines of text")
	fmt.Fprintln(w, "  rand     Generate random bytes/text")
	fmt.Fprintln(w, "  dig      DNS lookup utility")
	fmt.Fprintln(w, "  nslookup DNS lookup utility")
	fmt.Fprintln(w, "  head     Print the first lines of a file")
	fmt.Fprintln(w, "  tail     Print the last lines of a file")
	fmt.Fprintln(w, "  uniq     Filter adjacent matching lines")
	fmt.Fprintln(w, "  curl     Transfer data from a URL")
	fmt.Fprintln(w, "  wc       Print line, word, and byte counts")
	fmt.Fprintln(w, "  hex      Hex dump and encode/decode")
	fmt.Fprintln(w, "  base64   Base64 encode/decode")
	fmt.Fprintln(w, "  strings  Extract printable strings")
	fmt.Fprintln(w, "  diff     Compare files line by line")
	fmt.Fprintln(w, "  nc       Netcat - arbitrary TCP/UDP connections and listening")
	fmt.Fprintln(w, "  tw       Tiny web server for static files or benchmark")
	fmt.Fprintln(w, "  ifstat   Network interface statistics monitoring")
	fmt.Fprintln(w, "  np       Network ping/connectivity tool (TCP/UDP/ICMP/ARP/scan)")
	fmt.Fprintln(w, "  seq      Generate sequences of numbers")
	fmt.Fprintln(w, "  version  Print program version (-v, --version)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Common flags are implemented as a focused troubleshooting subset.")
}
