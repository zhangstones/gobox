package base

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

const goboxAliasType = "bash"

func init() {
	Register(NewCommand("alias", "Print bash alias or unalias shell code", aliasCmd))
}

func aliasCmd(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("alias", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	unalias := fs.Bool("u", false, "print unalias commands")
	help := fs.Bool("h", false, "show help")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeAliasUsage(stdout)
			return nil
		}
		return err
	}
	if *help {
		writeAliasUsage(stdout)
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	if *unalias {
		writeUnaliasScript(stdout)
		return nil
	}

	writeAliasScript(stdout)
	return nil
}

func writeAliasUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox alias [-u]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Print shell code for enabling or disabling gobox bash aliases.")
}

func writeAliasScript(w io.Writer) {
	fmt.Fprintln(w, "if [ -n \"${gobox_alias_type:-}\" ] && [ \"${gobox_alias_type}\" != \""+goboxAliasType+"\" ]; then")
	_, _ = io.WriteString(w, "  printf '%s\\n' \"gobox alias: expected gobox_alias_type to be empty or "+goboxAliasType+", got ${gobox_alias_type}\" >&2\n")
	fmt.Fprintln(w, "  false")
	fmt.Fprintln(w, "else")
	fmt.Fprintln(w, "  export gobox_alias_type="+goboxAliasType)
	for _, cmd := range Commands() {
		if cmd.Name() == "alias" {
			continue
		}
		fmt.Fprintf(w, "  alias %s='gobox %s'\n", cmd.Name(), cmd.Name())
	}
	fmt.Fprintln(w, "fi")
}

func writeUnaliasScript(w io.Writer) {
	fmt.Fprintln(w, "if [ -n \"${gobox_alias_type:-}\" ] && [ \"${gobox_alias_type}\" != \""+goboxAliasType+"\" ]; then")
	_, _ = io.WriteString(w, "  printf '%s\\n' \"gobox alias: expected gobox_alias_type to be empty or "+goboxAliasType+", got ${gobox_alias_type}\" >&2\n")
	fmt.Fprintln(w, "  false")
	fmt.Fprintln(w, "else")
	for _, cmd := range Commands() {
		if cmd.Name() == "alias" {
			continue
		}
		fmt.Fprintf(w, "  unalias %s 2>/dev/null || true\n", cmd.Name())
	}
	fmt.Fprintln(w, "  unset gobox_alias_type")
	fmt.Fprintln(w, "fi")
}
