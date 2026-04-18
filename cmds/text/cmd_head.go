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
		lines    = 10 // default number of lines
		bytes    = -1 // -1 means no byte limit
		quiet    = false
		showHelp = false
	)

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--lines":
			if i+1 >= len(args) {
				return fmt.Errorf("-n/--lines requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", args[i])
			}
			lines = n
		case strings.HasPrefix(arg, "-n="):
			n, err := strconv.Atoi(arg[3:])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", arg[3:])
			}
			lines = n
		case arg == "--lines=":
			if i+1 >= len(args) {
				return fmt.Errorf("--lines= requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", args[i])
			}
			lines = n
		case arg == "-c" || arg == "--bytes":
			if i+1 >= len(args) {
				return fmt.Errorf("-c/--bytes requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", args[i])
			}
			bytes = n
		case strings.HasPrefix(arg, "-c="):
			n, err := strconv.Atoi(arg[3:])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", arg[3:])
			}
			bytes = n
		case arg == "--bytes=":
			if i+1 >= len(args) {
				return fmt.Errorf("--bytes= requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of bytes: %s", args[i])
			}
			bytes = n
		case arg == "-q" || arg == "--quiet" || arg == "--silent":
			quiet = true
		case arg == "-h" || arg == "--help":
			showHelp = true
		case strings.HasPrefix(arg, "-"):
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
		if err := headReader(os.Stdin, os.Stdout, lines, bytes); err != nil {
			return err
		}
		return nil
	}

	// Process files
	for _, file := range files {
		if multipleFiles && !quiet {
			fmt.Printf("==> %s <==\n", file)
		}
		if err := headFile(file, os.Stdout, lines, bytes); err != nil {
			return err
		}
		if multipleFiles && !quiet && file != files[len(files)-1] {
			fmt.Println()
		}
	}

	return nil
}

func printHeadUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox head [OPTIONS] [FILE...]")
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

func headReader(r io.Reader, w io.Writer, lines int, bytes int) error {
	if bytes >= 0 {
		// Byte mode
		return headBytes(r, w, bytes)
	}
	// Line mode
	return headLines(r, w, lines)
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

func headFile(filename string, w io.Writer, lines int, bytes int) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return headReader(file, w, lines, bytes)
}

