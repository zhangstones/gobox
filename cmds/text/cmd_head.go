package text

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// headCmd implements the head command
func HeadCmd(args []string) error {
	var (
		lines        = 10    // default number of lines
		linesFromEnd = false // -n -N: print all but the last N lines
		bytes        = -1    // -1 means no byte limit
		bytesFromEnd = false // -c -N: print all but the last N bytes
		quiet        = false
		showHelp     = false
	)

	// parseCount parses a NUM value; a leading '-' selects "all but last N" mode.
	parseCount := func(s, kind string) (n int, fromEnd bool, err error) {
		v, convErr := strconv.Atoi(s)
		if convErr != nil {
			return 0, false, fmt.Errorf("invalid number of %s: %s", kind, s)
		}
		if v < 0 {
			return -v, true, nil
		}
		return v, false, nil
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--lines":
			if i+1 >= len(args) {
				return fmt.Errorf("-n/--lines requires an argument")
			}
			i++
			n, fe, err := parseCount(args[i], "lines")
			if err != nil {
				return err
			}
			lines, linesFromEnd = n, fe
		case strings.HasPrefix(arg, "-n="):
			n, fe, err := parseCount(arg[3:], "lines")
			if err != nil {
				return err
			}
			lines, linesFromEnd = n, fe
		case strings.HasPrefix(arg, "--lines="):
			n, fe, err := parseCount(arg[len("--lines="):], "lines")
			if err != nil {
				return err
			}
			lines, linesFromEnd = n, fe
		case strings.HasPrefix(arg, "-n") && arg != "-n":
			// GNU-style attached value, e.g. -n5 or -n-2
			n, fe, err := parseCount(arg[2:], "lines")
			if err != nil {
				return err
			}
			lines, linesFromEnd = n, fe
		case arg == "-c" || arg == "--bytes":
			if i+1 >= len(args) {
				return fmt.Errorf("-c/--bytes requires an argument")
			}
			i++
			n, fe, err := parseCount(args[i], "bytes")
			if err != nil {
				return err
			}
			bytes, bytesFromEnd = n, fe
		case strings.HasPrefix(arg, "-c="):
			n, fe, err := parseCount(arg[3:], "bytes")
			if err != nil {
				return err
			}
			bytes, bytesFromEnd = n, fe
		case strings.HasPrefix(arg, "--bytes="):
			n, fe, err := parseCount(arg[len("--bytes="):], "bytes")
			if err != nil {
				return err
			}
			bytes, bytesFromEnd = n, fe
		case strings.HasPrefix(arg, "-c") && arg != "-c":
			// GNU-style attached value, e.g. -c100 or -c-100
			n, fe, err := parseCount(arg[2:], "bytes")
			if err != nil {
				return err
			}
			bytes, bytesFromEnd = n, fe
		case arg == "-q" || arg == "--quiet" || arg == "--silent":
			quiet = true
		case arg == "-h" || arg == "--help":
			showHelp = true
		case len(arg) > 1 && strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}

doneFlags:
	if showHelp {
		printHeadUsage(os.Stdout)
		return nil
	}

	files := args[i:]
	multipleFiles := len(files) > 1

	// If no files, read from stdin
	if len(files) == 0 {
		if err := headReader(os.Stdin, os.Stdout, lines, linesFromEnd, bytes, bytesFromEnd); err != nil {
			return err
		}
		return nil
	}

	// Process files
	for _, file := range files {
		if multipleFiles && !quiet {
			fmt.Printf("==> %s <==\n", file)
		}
		if err := headFile(file, os.Stdout, lines, linesFromEnd, bytes, bytesFromEnd); err != nil {
			return err
		}
		if multipleFiles && !quiet && file != files[len(files)-1] {
			fmt.Println()
		}
	}

	return nil
}

func printHeadUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox head [OPTION]... [FILE...]")
	fmt.Fprintln(w, "Print the first lines of a file.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n NUM, --lines=NUM   Print the first NUM lines (default 10)")
	fmt.Fprintln(w, "  -c NUM, --bytes=NUM   Print the first NUM bytes")
	fmt.Fprintln(w, "  -q, --quiet           Never print headers giving file names")
	fmt.Fprintln(w, "  -h, --help            Show this help message")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox head file.txt           Print first 10 lines")
	fmt.Fprintln(w, "  gobox head -n 20 file.txt     Print first 20 lines")
	fmt.Fprintln(w, "  gobox head -c 100 file.txt    Print first 100 bytes")
	fmt.Fprintln(w, "  cat file.txt | gobox head -n 5")
}

func headReader(r io.Reader, w io.Writer, lines int, linesFromEnd bool, bytes int, bytesFromEnd bool) error {
	if bytes >= 0 {
		// Byte mode
		if bytesFromEnd {
			return headBytesAllBut(r, w, bytes)
		}
		return headBytes(r, w, bytes)
	}
	// Line mode
	if linesFromEnd {
		return headLinesAllBut(r, w, lines)
	}
	return headLines(r, w, lines)
}

// headLinesAllBut prints every line except the last n (GNU head -n -N).
func headLinesAllBut(r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	var buffered []string
	for scanner.Scan() {
		buffered = append(buffered, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for i := 0; i < len(buffered)-n; i++ {
		fmt.Fprintln(w, buffered[i])
	}
	return nil
}

// headBytesAllBut writes every byte except the last n (GNU head -c -N).
func headBytesAllBut(r io.Reader, w io.Writer, n int) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if n >= len(data) {
		return nil
	}
	_, err = w.Write(data[:len(data)-n])
	return err
}

func headLines(r io.Reader, w io.Writer, n int) error {
	scanner := bufio.NewScanner(r)
	line := 0
	for scanner.Scan() {
		if line >= n {
			break
		}
		fmt.Fprintln(w, scanner.Text())
		line++
	}
	return scanner.Err()
}

func headBytes(r io.Reader, w io.Writer, n int) error {
	reader := io.LimitReader(r, int64(n))
	_, err := io.Copy(w, reader)
	return err
}

func headFile(filename string, w io.Writer, lines int, linesFromEnd bool, bytes int, bytesFromEnd bool) error {
	if filename == "-" {
		return headReader(os.Stdin, w, lines, linesFromEnd, bytes, bytesFromEnd)
	}
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return headReader(file, w, lines, linesFromEnd, bytes, bytesFromEnd)
}
