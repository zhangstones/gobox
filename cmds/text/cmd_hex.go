package text

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func HexCmd(args []string) error {
	fsFlags := flag.NewFlagSet("hex", flag.ContinueOnError)
	dump := fsFlags.Bool("dump", false, "dump bytes like hexdump")
	encode := fsFlags.Bool("encode", false, "encode input as hex")
	decode := fsFlags.Bool("decode", false, "decode hex input")
	canonical := fsFlags.Bool("C", false, "canonical dump")
	limit := fsFlags.Int64("n", -1, "read at most LEN bytes")
	offset := fsFlags.Int64("s", 0, "skip OFFSET bytes")
	verbose := fsFlags.Bool("v", false, "do not fold repeated lines")
	format := fsFlags.String("e", "", "format string subset")
	output := fsFlags.String("o", "", "write output to file")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox hex --dump|--encode|--decode [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Encode, decode, or dump hexadecimal data.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Modes:")
		fmt.Fprintln(os.Stderr, "  --dump              dump bytes like hexdump")
		fmt.Fprintln(os.Stderr, "  --encode            encode input as hex")
		fmt.Fprintln(os.Stderr, "  --decode            decode hex input")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -C                  canonical dump")
		fmt.Fprintln(os.Stderr, "  -n LEN              read at most LEN bytes")
		fmt.Fprintln(os.Stderr, "  -s OFFSET           skip OFFSET bytes")
		fmt.Fprintln(os.Stderr, "  -v                  do not fold repeated lines")
		fmt.Fprintln(os.Stderr, "  -e FORMAT           format string subset for dump mode")
		fmt.Fprintln(os.Stderr, "  -o FILE             write output to file")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	modes := 0
	for _, enabled := range []bool{*dump, *encode, *decode} {
		if enabled {
			modes++
		}
	}
	if modes != 1 {
		return fmt.Errorf("exactly one of --dump, --encode, --decode is required")
	}
	files := fsFlags.Args()
	data, err := readAllInputs(files)
	if err != nil {
		return err
	}
	if *offset > 0 {
		if *offset > int64(len(data)) {
			data = nil
		} else {
			data = data[*offset:]
		}
	}
	if *limit >= 0 && *limit < int64(len(data)) {
		data = data[:*limit]
	}
	var out io.Writer = os.Stdout
	var f *os.File
	if *output != "" {
		f, err = os.Create(*output)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	switch {
	case *encode:
		_, err = fmt.Fprintln(out, hex.EncodeToString(data))
	case *decode:
		clean := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, string(data))
		var decoded []byte
		decoded, err = hex.DecodeString(clean)
		if err == nil {
			_, err = out.Write(decoded)
		}
	case *dump:
		if *format != "" {
			err = dumpHexFormat(out, data, *format)
		} else {
			_ = canonical
			err = dumpCanonicalHex(out, data, *offset, !*verbose)
		}
	}
	return err
}

func readAllInputs(files []string) ([]byte, error) {
	if len(files) == 0 {
		return io.ReadAll(os.Stdin)
	}
	var out []byte
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		out = append(out, b...)
	}
	return out, nil
}

func dumpCanonicalHex(w io.Writer, data []byte, base int64, fold bool) error {
	var prevChunk []byte
	havePrevFull := false
	starPrinted := false
	for off := 0; off < len(data); off += 16 {
		end := off + 16
		if end > len(data) {
			end = len(data)
		}
		chunk := data[off:end]
		isFullRow := len(chunk) == 16

		if fold && isFullRow && havePrevFull && bytes.Equal(chunk, prevChunk) {
			if !starPrinted {
				fmt.Fprintln(w, "*")
				starPrinted = true
			}
			continue
		}

		fmt.Fprintf(w, "%08x  ", int64(off)+base)
		for i := 0; i < 16; i++ {
			if i < len(chunk) {
				fmt.Fprintf(w, "%02x ", chunk[i])
			} else {
				fmt.Fprint(w, "   ")
			}
			if i == 7 {
				fmt.Fprint(w, " ")
			}
		}
		fmt.Fprint(w, " |")
		for _, b := range chunk {
			if b >= 32 && b <= 126 {
				fmt.Fprintf(w, "%c", b)
			} else {
				fmt.Fprint(w, ".")
			}
		}
		fmt.Fprintln(w, "|")

		if isFullRow {
			prevChunk = append([]byte(nil), chunk...)
		}
		havePrevFull = isFullRow
		starPrinted = false
	}
	fmt.Fprintf(w, "%08x\n", int64(len(data))+base)
	return nil
}

func dumpHexFormat(w io.Writer, data []byte, format string) error {
	// Deliberately small subset: byte-oriented hex/decimal formats commonly used in tests.
	if strings.Contains(format, "%02x") || strings.Contains(format, "%02X") {
		upper := strings.Contains(format, "%02X")
		for _, b := range data {
			if upper {
				fmt.Fprintf(w, "%02X", b)
			} else {
				fmt.Fprintf(w, "%02x", b)
			}
		}
		fmt.Fprintln(w)
		return nil
	}
	if strings.Contains(format, "%u") || strings.Contains(format, "%d") {
		for i, b := range data {
			if i > 0 {
				fmt.Fprint(w, " ")
			}
			fmt.Fprint(w, strconv.Itoa(int(b)))
		}
		fmt.Fprintln(w)
		return nil
	}
	return fmt.Errorf("unsupported hexdump format subset: %s", format)
}
