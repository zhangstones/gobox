package fs

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type readpathExitError struct{}

func (readpathExitError) Error() string          { return "some paths could not be resolved" }
func (readpathExitError) ExitCode() int          { return 1 }
func (readpathExitError) SuppressCLIError() bool { return true }

func ReadpathCmd(args []string) error {
	fsFlags := flag.NewFlagSet("readpath", flag.ContinueOnError)
	canonicalize := fsFlags.Bool("f", false, "canonicalize by following symlinks")
	fsFlags.BoolVar(canonicalize, "canonicalize", false, "canonicalize by following symlinks")
	mustExist := fsFlags.Bool("e", false, "all path components must exist")
	fsFlags.BoolVar(mustExist, "canonicalize-existing", false, "all path components must exist")
	allowMissing := fsFlags.Bool("m", false, "allow missing path components")
	fsFlags.BoolVar(allowMissing, "canonicalize-missing", false, "allow missing path components")
	readlinkMode := fsFlags.Bool("l", false, "read symlink target")
	fsFlags.BoolVar(readlinkMode, "readlink", false, "read symlink target")
	noNewline := fsFlags.Bool("n", false, "do not print trailing newline")
	fsFlags.BoolVar(noNewline, "no-newline", false, "do not print trailing newline")
	quiet := fsFlags.Bool("q", false, "suppress most error messages")
	fsFlags.BoolVar(quiet, "quiet", false, "suppress most error messages")
	zero := fsFlags.Bool("z", false, "end each output line with NUL")
	fsFlags.BoolVar(zero, "zero", false, "end each output line with NUL")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox readpath [OPTION]... FILE...")
		fmt.Fprintln(os.Stderr, "Resolve or inspect pathnames.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Modes:")
		fmt.Fprintln(os.Stderr, "  -f, --canonicalize             canonicalize by following symlinks")
		fmt.Fprintln(os.Stderr, "  -e, --canonicalize-existing    require all path components to exist")
		fmt.Fprintln(os.Stderr, "  -m, --canonicalize-missing     allow missing path components")
		fmt.Fprintln(os.Stderr, "  -l, --readlink                 print symlink target instead of canonical path")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -n, --no-newline               do not print trailing newline")
		fmt.Fprintln(os.Stderr, "  -z, --zero                     terminate each output with NUL")
		fmt.Fprintln(os.Stderr, "  -q, --quiet                    suppress most error messages")
		fmt.Fprintln(os.Stderr, "  -h, --help                     show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	paths := fsFlags.Args()
	if len(paths) == 0 {
		return fmt.Errorf("missing operand")
	}
	sep := "\n"
	if *zero {
		sep = "\x00"
	}
	var hadErr bool
	for _, p := range paths {
		out, err := resolveReadpath(p, *readlinkMode, *canonicalize, *mustExist, *allowMissing)
		if err != nil {
			hadErr = true
			if !*quiet {
				fmt.Fprintf(os.Stderr, "readpath: %s: %v\n", p, err)
			}
			continue
		}
		fmt.Fprint(os.Stdout, out)
		if !*noNewline || *zero || len(paths) > 1 {
			fmt.Fprint(os.Stdout, sep)
		}
	}
	if hadErr {
		return readpathExitError{}
	}
	return nil
}

func resolveReadpath(p string, readlinkMode, canonicalize, mustExist, allowMissing bool) (string, error) {
	if readlinkMode {
		return os.Readlink(p)
	}
	if allowMissing {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		return filepath.Clean(abs), nil
	}
	if mustExist {
		if _, err := os.Stat(p); err != nil {
			return "", err
		}
	}
	// Default/-f mode: GNU realpath's "all but last component must exist" — fall back to resolved-parent + literal base.
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return filepath.Abs(resolved)
	}
	parent := filepath.Dir(p)
	base := filepath.Base(p)
	if parent == "." || parent == "" {
		parent = "."
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(resolvedParent, base))
}
