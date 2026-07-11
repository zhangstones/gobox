package text

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode"
)

func StringsCmd(args []string) error {
	fsFlags := flag.NewFlagSet("strings", flag.ContinueOnError)
	minLen := fsFlags.Int("n", 4, "minimum string length")
	withFile := fsFlags.Bool("f", false, "print filename before each string")
	offsetBase := fsFlags.String("t", "", "print offset in o, d, or x")
	_ = fsFlags.Bool("a", false, "scan entire file")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox strings [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Print printable character sequences from files.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -n N       minimum string length")
		fmt.Fprintln(os.Stderr, "  -f         print filename before each string")
		fmt.Fprintln(os.Stderr, "  -t BASE    print offset in o, d, or x")
		fmt.Fprintln(os.Stderr, "  -a         scan entire file")
		fmt.Fprintln(os.Stderr, "  -h, --help show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *offsetBase != "" && *offsetBase != "o" && *offsetBase != "d" && *offsetBase != "x" {
		return fmt.Errorf("unsupported offset base %q", *offsetBase)
	}
	files := fsFlags.Args()
	if len(files) == 0 {
		return stringsFromReader("-", os.Stdin, *minLen, *withFile, *offsetBase)
	}
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		err = stringsFromReader(file, f, *minLen, *withFile, *offsetBase)
		_ = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func stringsFromReader(name string, r io.Reader, minLen int, withFile bool, offsetBase string) error {
	br := bufio.NewReader(r)
	var buf []rune
	var start, offset int64

	print := func() {
		if len(buf) < minLen {
			return
		}
		if withFile {
			fmt.Printf("%s: ", name)
		}
		if offsetBase != "" {
			switch offsetBase {
			case "o":
				fmt.Printf("%7s ", strconv.FormatInt(start, 8))
			case "d":
				fmt.Printf("%7d ", start)
			case "x":
				fmt.Printf("%7s ", strconv.FormatInt(start, 16))
			}
		}
		fmt.Println(string(buf))
	}

	for {
		b, err := br.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		ch := rune(b)
		if unicode.IsPrint(ch) && b < 0x80 {
			if len(buf) == 0 {
				start = offset
			}
			buf = append(buf, ch)
		} else {
			print()
			buf = buf[:0]
		}
		offset++
	}
	print()
	return nil
}
