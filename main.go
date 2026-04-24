package main

import (
	"fmt"
	"io"
	"os"

	"gobox/cmds/base"
	_ "gobox/cmds/disk"
	_ "gobox/cmds/fs"
	_ "gobox/cmds/net"
	_ "gobox/cmds/proc"
	_ "gobox/cmds/text"
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

	switch cmd {
	case "--help", "-h", "help":
		usage(stdout)
		return 0
	case "--version", "version", "-v":
		fmt.Fprintln(stdout, "gobox 0.1 - container troubleshooting toolset")
		return 0
	}

	command, ok := base.Lookup(cmd)
	if !ok {
		fmt.Fprintln(stderr, "unknown command:", cmd)
		usage(stdout)
		return 127
	}

	err := command.Run(args, stdout)
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
	for _, cmd := range base.Commands() {
		fmt.Fprintf(w, "  %-12s %s\n", cmd.Name(), cmd.Help())
	}
	fmt.Fprintf(w, "  %-12s %s\n", "version", "Print program version (-v, --version)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Common flags are implemented as a focused troubleshooting subset.")
}
