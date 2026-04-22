package text

import (
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
		fsFlags.PrintDefaults()
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
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	start := 0
	buf := make([]rune, 0)
	flush := func(end int) {
		if len(buf) < minLen {
			return
		}
		if withFile {
			fmt.Printf("%s: ", name)
		}
		if offsetBase != "" {
			switch offsetBase {
			case "o":
				fmt.Printf("%7s ", strconv.FormatInt(int64(start), 8))
			case "d":
				fmt.Printf("%7d ", start)
			case "x":
				fmt.Printf("%7s ", strconv.FormatInt(int64(start), 16))
			}
		}
		fmt.Println(string(buf))
		_ = end
	}
	for i, b := range data {
		r := rune(b)
		if unicode.IsPrint(r) && b < 0x80 {
			if len(buf) == 0 {
				start = i
			}
			buf = append(buf, r)
		} else {
			flush(i)
			buf = buf[:0]
		}
	}
	flush(len(data))
	return nil
}
