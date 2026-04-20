package text

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// wcFlags holds the command-line flags for wc
type wcFlags struct {
	lines      bool
	words      bool
	bytes      bool
	chars      bool
	maxLineLen bool
}

// wcResult holds the count results for a file
type wcResult struct {
	lines      int64
	words      int64
	bytes      int64
	chars      int64
	maxLineLen int64
	filename   string
}

func WcCmd(args []string) error {
	flags := wcFlags{}
	showHelp := false

	// Parse flags
	argIndex := 0
	for argIndex < len(args) {
		arg := args[argIndex]
		switch {
		case arg == "-l" || arg == "--lines":
			flags.lines = true
			argIndex++
		case arg == "-w" || arg == "--words":
			flags.words = true
			argIndex++
		case arg == "-c" || arg == "--bytes":
			flags.bytes = true
			argIndex++
		case arg == "-m" || arg == "--chars":
			flags.chars = true
			argIndex++
		case arg == "-L" || arg == "--max-line-length":
			flags.maxLineLen = true
			argIndex++
		case arg == "-h" || arg == "--help":
			showHelp = true
			argIndex++
		case strings.HasPrefix(arg, "-") && len(arg) > 2:
			// Handle combined short flags like -lw
			for _, c := range arg[1:] {
				switch c {
				case 'l':
					flags.lines = true
				case 'w':
					flags.words = true
				case 'c':
					flags.bytes = true
				case 'm':
					flags.chars = true
				case 'L':
					flags.maxLineLen = true
				default:
					return fmt.Errorf("unknown option: -%c", c)
				}
			}
			argIndex++
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return fmt.Errorf("unknown option: %s", arg)
			}
			// Not a flag, must be a filename
			goto doneFlags
		}
	}

doneFlags:
	// Remaining args are files
	files := args[argIndex:]

	if showHelp {
		printWcUsage(os.Stdout)
		return nil
	}

	// If no flags specified, show all counts
	showAll := !flags.lines && !flags.words && !flags.bytes && !flags.chars && !flags.maxLineLen

	var totalResult wcResult
	var results []wcResult

	// If no files, read from stdin
	if len(files) == 0 {
		result, err := wcReader(os.Stdin, "")
		if err != nil {
			return err
		}
		results = append(results, result)
	} else {
		for _, filename := range files {
			result, err := wcFile(filename)
			if err != nil {
				return err
			}
			results = append(results, result)
			totalResult.lines += result.lines
			totalResult.words += result.words
			totalResult.bytes += result.bytes
			totalResult.chars += result.chars
			if result.maxLineLen > totalResult.maxLineLen {
				totalResult.maxLineLen = result.maxLineLen
			}
		}
	}

	// Output results
	for _, result := range results {
		printWcResult(os.Stdout, result, flags, showAll)
	}

	// Print total line if multiple files
	if len(results) > 1 {
		totalResult.filename = "total"
		printWcResult(os.Stdout, totalResult, flags, showAll)
	}

	return nil
}

func wcFile(filename string) (wcResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return wcResult{}, fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return wcReader(file, filename)
}

func wcReader(r io.Reader, filename string) (wcResult, error) {
	scanner := bufio.NewReader(r)
	var result wcResult
	result.filename = filename

	lineLen := int64(0)
	inWord := false

	for {
		b, err := scanner.ReadByte()
		if err != nil && err != io.EOF {
			return result, err
		}

		if err == io.EOF {
			break
		}

		result.bytes++
		if b < utf8.RuneSelf {
			result.chars++
		} else {
			buf := []byte{b}
			for !utf8.FullRune(buf) {
				next, nextErr := scanner.ReadByte()
				if nextErr != nil {
					break
				}
				buf = append(buf, next)
				result.bytes++
			}
			result.chars++
		}

		if b == '\n' {
			if inWord {
				result.words++
			}
			result.lines++
			if lineLen > result.maxLineLen {
				result.maxLineLen = lineLen
			}
			lineLen = 0
			inWord = false
		} else if unicode.IsSpace(rune(b)) {
			if inWord {
				result.words++
				inWord = false
			}
			lineLen++
		} else {
			lineLen++
			inWord = true
		}
	}

	// Handle last word if file doesn't end with whitespace
	if inWord {
		result.words++
	}

	// Handle last line without newline
	if lineLen > 0 {
		if lineLen > result.maxLineLen {
			result.maxLineLen = lineLen
		}
	}

	return result, nil
}

func printWcResult(w io.Writer, result wcResult, flags wcFlags, showAll bool) {
	filename := result.filename
	if filename == "" {
		filename = "-"
	}

	if showAll {
		// Default format: lines words bytes filename
		fmt.Fprintf(w, "%7d %7d %7d %s\n", result.lines, result.words, result.bytes, filename)
		return
	}

	// When specific flags are used, show selected counts on one line with filename at end
	// Format: count1 count2 ... filename
	hasFilename := false
	if flags.lines {
		fmt.Fprintf(w, "%7d", result.lines)
		hasFilename = true
	}
	if flags.words {
		fmt.Fprintf(w, "%7d", result.words)
		hasFilename = true
	}
	if flags.bytes {
		fmt.Fprintf(w, "%7d", result.bytes)
		hasFilename = true
	}
	if flags.chars {
		fmt.Fprintf(w, "%7d", result.chars)
		hasFilename = true
	}
	if flags.maxLineLen {
		fmt.Fprintf(w, "%7d", result.maxLineLen)
		hasFilename = true
	}

	// If no specific flags matched, show all
	if !hasFilename {
		fmt.Fprintf(w, "%7d %7d %7d", result.lines, result.words, result.bytes)
	}

	fmt.Fprintf(w, " %s\n", filename)
}

func printWcUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox wc [OPTION]... [FILE]...")
	fmt.Fprintln(w, "Print line, word, and byte counts for each FILE.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -l, --lines         print the line counts")
	fmt.Fprintln(w, "  -w, --words         print the word counts")
	fmt.Fprintln(w, "  -c, --bytes         print the byte counts")
	fmt.Fprintln(w, "  -m, --chars         print the character counts")
	fmt.Fprintln(w, "  -L, --max-line-length")
	fmt.Fprintln(w, "                      print the maximum line length")
	fmt.Fprintln(w, "  -h, --help          display this help and exit")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "With no FILE, or when FILE is -, read standard input.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox wc file.txt              Show lines, words, bytes for file")
	fmt.Fprintln(w, "  gobox wc -l file.txt           Show only line count")
	fmt.Fprintln(w, "  gobox wc -lw file.txt          Show lines and words only")
	fmt.Fprintln(w, "  cat file.txt | gobox wc        Count from stdin")
	fmt.Fprintln(w, "  gobox wc file1.txt file2.txt   Show counts for each file with total")
}
