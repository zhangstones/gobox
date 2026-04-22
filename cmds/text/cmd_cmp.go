package text

import (
	"flag"
	"fmt"
	"io"
	"os"
)

type cmpExitError int

func (e cmpExitError) Error() string { return fmt.Sprintf("exit code %d", int(e)) }
func (e cmpExitError) ExitCode() int { return int(e) }

func CmpCmd(args []string) error {
	fsFlags := flag.NewFlagSet("cmp", flag.ContinueOnError)
	silent := fsFlags.Bool("s", false, "silent")
	list := fsFlags.Bool("l", false, "list all differing bytes")
	limit := fsFlags.Int64("n", -1, "compare at most NUM bytes")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox cmp [OPTION]... FILE1 FILE2")
		fsFlags.PrintDefaults()
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	files := fsFlags.Args()
	if len(files) != 2 {
		return fmt.Errorf("cmp requires two operands")
	}
	if files[0] == "-" && files[1] == "-" {
		return fmt.Errorf("cmp: both operands cannot be standard input")
	}
	r1, c1, err := openCmpInput(files[0])
	if err != nil {
		return err
	}
	defer c1()
	r2, c2, err := openCmpInput(files[1])
	if err != nil {
		return err
	}
	defer c2()
	equal, err := compareStreams(r1, r2, *limit, *silent, *list, files[0], files[1])
	if err != nil {
		return err
	}
	if !equal {
		return cmpExitError(1)
	}
	return nil
}

func openCmpInput(name string) (io.Reader, func(), error) {
	if name == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func compareStreams(a, b io.Reader, limit int64, silent, list bool, nameA, nameB string) (bool, error) {
	ba := make([]byte, 1)
	bb := make([]byte, 1)
	pos := int64(1)
	line := int64(1)
	equal := true
	for limit < 0 || pos <= limit {
		na, ea := a.Read(ba)
		nb, eb := b.Read(bb)
		if na == 0 && nb == 0 {
			if ea == io.EOF && eb == io.EOF {
				break
			}
			if ea != nil && ea != io.EOF {
				return false, ea
			}
			if eb != nil && eb != io.EOF {
				return false, eb
			}
		}
		if na != nb || (na == 1 && nb == 1 && ba[0] != bb[0]) {
			equal = false
			if !silent {
				if list && na == 1 && nb == 1 {
					fmt.Printf("%d %o %o\n", pos, ba[0], bb[0])
				} else {
					fmt.Printf("%s %s differ: byte %d, line %d\n", nameA, nameB, pos, line)
					return false, nil
				}
			}
		}
		if na == 1 && ba[0] == '\n' {
			line++
		}
		if ea == io.EOF || eb == io.EOF {
			break
		}
		pos++
	}
	return equal, nil
}
