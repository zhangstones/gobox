package text

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func Base64Cmd(args []string) error {
	fsFlags := flag.NewFlagSet("base64", flag.ContinueOnError)
	decode := fsFlags.Bool("d", false, "decode data")
	fsFlags.BoolVar(decode, "decode", false, "decode data")
	wrap := fsFlags.Int("w", 76, "wrap encoded lines after COLS characters (0 disables wrapping)")
	fsFlags.IntVar(wrap, "wrap", 76, "wrap encoded lines after COLS characters (0 disables wrapping)")
	ignoreGarbage := fsFlags.Bool("i", false, "ignore non-alphabet characters while decoding")
	fsFlags.BoolVar(ignoreGarbage, "ignore-garbage", false, "ignore non-alphabet characters while decoding")
	output := fsFlags.String("o", "", "write output to file")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox base64 [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Encode or decode base64 data.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -d, --decode              decode data")
		fmt.Fprintln(os.Stderr, "  -w, --wrap COLS           wrap encoded lines after COLS characters (0 disables wrapping)")
		fmt.Fprintln(os.Stderr, "  -i, --ignore-garbage      ignore non-alphabet characters while decoding")
		fmt.Fprintln(os.Stderr, "  -o FILE                   write output to file")
		fmt.Fprintln(os.Stderr, "  -h, --help                show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	var out io.Writer = os.Stdout
	var outFile *os.File
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer f.Close()
		outFile = f
		out = outFile
	}
	data, err := readAllInputs(fsFlags.Args())
	if err != nil {
		return err
	}
	return base64Bytes(data, out, *decode, *ignoreGarbage, *wrap)
}

func base64Stream(r io.Reader, w io.Writer, decode, ignoreGarbage bool, wrap int) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return base64Bytes(data, w, decode, ignoreGarbage, wrap)
}

func base64Bytes(data []byte, w io.Writer, decode, ignoreGarbage bool, wrap int) error {
	if decode {
		s := string(data)
		if ignoreGarbage {
			var b strings.Builder
			for _, r := range s {
				if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '+' || r == '/' || r == '=' {
					b.WriteRune(r)
				}
			}
			s = b.String()
		}
		s = strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
				return -1
			}
			return r
		}, s)
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}
		_, err = w.Write(decoded)
		return err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	if wrap == 0 {
		_, err := fmt.Fprint(w, encoded)
		return err
	}
	if wrap > 0 {
		for len(encoded) > wrap {
			fmt.Fprintln(w, encoded[:wrap])
			encoded = encoded[wrap:]
		}
	}
	if encoded != "" {
		fmt.Fprintln(w, encoded)
	}
	return nil
}
