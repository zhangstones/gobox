package fs

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func TruncateCmd(args []string) error {
	fsFlags := flag.NewFlagSet("truncate", flag.ContinueOnError)
	sizeArg := fsFlags.String("s", "", "set or adjust file size")
	noCreate := fsFlags.Bool("c", false, "do not create files")
	fsFlags.BoolVar(noCreate, "no-create", false, "do not create files")
	ref := fsFlags.String("r", "", "use reference file size")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox truncate -s SIZE FILE... | gobox truncate -r RFILE FILE...")
		fmt.Fprintln(os.Stderr, "Shrink or extend files to a specified size.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -s SIZE             set or adjust file size")
		fmt.Fprintln(os.Stderr, "  -r RFILE            use reference file size")
		fmt.Fprintln(os.Stderr, "  -c, --no-create     do not create files")
		fmt.Fprintln(os.Stderr, "  -h, --help          show this help")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox truncate -s 0 app.log")
		fmt.Fprintln(os.Stderr, "  gobox truncate -r ref.bin copy.bin")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	files := fsFlags.Args()
	if len(files) == 0 {
		return fmt.Errorf("missing file operand")
	}
	var refSize int64
	if *ref != "" {
		info, err := os.Stat(*ref)
		if err != nil {
			return err
		}
		refSize = info.Size()
	} else if *sizeArg == "" {
		return fmt.Errorf("missing -s SIZE or -r RFILE")
	}
	for _, file := range files {
		targetSize := refSize
		if *ref == "" {
			current := int64(0)
			if info, statErr := os.Stat(file); statErr == nil {
				current = info.Size()
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
			parsed, relative, err := parseTruncateSize(*sizeArg)
			if err != nil {
				return err
			}
			if relative {
				targetSize = current + parsed
			} else {
				targetSize = parsed
			}
		}
		if targetSize < 0 {
			targetSize = 0
		}
		if *noCreate {
			if _, err := os.Stat(file); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
		}
		if !*noCreate {
			f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY, 0o666)
			if err != nil {
				return err
			}
			_ = f.Close()
		}
		if err := os.Truncate(file, targetSize); err != nil {
			return err
		}
	}
	return nil
}

func parseTruncateSize(s string) (int64, bool, error) {
	relative := false
	sign := int64(1)
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		relative = true
		if s[0] == '-' {
			sign = -1
		}
		s = s[1:]
	}
	mult := int64(1)
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'K', 'k':
			mult = 1024
			s = s[:len(s)-1]
		case 'M', 'm':
			mult = 1024 * 1024
			s = s[:len(s)-1]
		case 'G', 'g':
			mult = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid size %q", s)
	}
	return sign * n * mult, relative, nil
}
