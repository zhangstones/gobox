package text

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// uniqCmd implements the uniq command for filtering adjacent duplicate lines
func UniqCmd(args []string) error {
	var (
		showCount     bool
		showRepeated  bool
		showUnique    bool
		ignoreCase    bool
		checkChars    int
		skipFields    int
		showHelp      bool
	)

	// Parse flags
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-c" || arg == "--count":
			showCount = true
		case arg == "-d" || arg == "--repeated":
			showRepeated = true
		case arg == "-u" || arg == "--unique":
			showUnique = true
		case arg == "-i" || arg == "--ignore-case":
			ignoreCase = true
		case arg == "-h" || arg == "--help":
			showHelp = true
		case strings.HasPrefix(arg, "-w") || strings.HasPrefix(arg, "--check-chars"):
			// Handle -w NUM or --check-chars=NUM
			var numStr string
			if strings.HasPrefix(arg, "-w") {
				if len(arg) > 2 {
					numStr = arg[2:]
				} else if i+1 < len(args) {
					i++
					numStr = args[i]
				} else {
					return fmt.Errorf("-w requires an argument")
				}
			} else { // --check-chars
				if strings.Contains(arg, "=") {
					numStr = strings.SplitN(arg, "=", 2)[1]
				} else if i+1 < len(args) {
					i++
					numStr = args[i]
				} else {
					return fmt.Errorf("--check-chars requires an argument")
				}
			}
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 0 {
				return fmt.Errorf("invalid number: %s", numStr)
			}
			checkChars = num
		case strings.HasPrefix(arg, "-f") || strings.HasPrefix(arg, "--skip-fields"):
			// Handle -f NUM or --skip-fields=NUM
			var numStr string
			if strings.HasPrefix(arg, "-f") {
				if len(arg) > 2 {
					numStr = arg[2:]
				} else if i+1 < len(args) {
					i++
					numStr = args[i]
				} else {
					return fmt.Errorf("-f requires an argument")
				}
			} else { // --skip-fields
				if strings.Contains(arg, "=") {
					numStr = strings.SplitN(arg, "=", 2)[1]
				} else if i+1 < len(args) {
					i++
					numStr = args[i]
				} else {
					return fmt.Errorf("--skip-fields requires an argument")
				}
			}
			num, err := strconv.Atoi(numStr)
			if err != nil || num < 0 {
				return fmt.Errorf("invalid number: %s", numStr)
			}
			skipFields = num
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}

doneFlags:
	remaining := args[i:]

	if showHelp {
		printUniqUsage(os.Stdout)
		return nil
	}

	// If no files specified, read from stdin
	if len(remaining) == 0 {
		return uniqReader(os.Stdin, os.Stdout, showCount, showRepeated, showUnique, ignoreCase, checkChars, skipFields)
	}

	// Process files
	for _, file := range remaining {
		if err := uniqFile(file, os.Stdout, showCount, showRepeated, showUnique, ignoreCase, checkChars, skipFields); err != nil {
			return err
		}
	}

	return nil
}

func printUniqUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox uniq [OPTION]... [FILE]")
	fmt.Fprintln(w, "Filter adjacent matching lines from FILE (or stdin)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -c, --count          Prefix lines by the number of occurrences")
	fmt.Fprintln(w, "  -d, --repeated       Only print duplicate lines")
	fmt.Fprintln(w, "  -u, --unique         Only print unique lines")
	fmt.Fprintln(w, "  -i, --ignore-case    Ignore case differences")
	fmt.Fprintln(w, "  -w, --check-chars=N  Compare at most N characters")
	fmt.Fprintln(w, "  -f, --skip-fields=N  Skip the first N fields")
	fmt.Fprintln(w, "  -h, --help           Show this help message")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Note: uniq only works on sorted input (adjacent identical lines)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox uniq file.txt")
	fmt.Fprintln(w, "  cat file.txt | gobox uniq")
	fmt.Fprintln(w, "  gobox uniq -c file.txt")
	fmt.Fprintln(w, "  gobox uniq -d file.txt")
	fmt.Fprintln(w, "  gobox uniq -i file.txt")
	fmt.Fprintln(w, "  gobox uniq -w 5 file.txt")
	fmt.Fprintln(w, "  gobox uniq -f 2 file.txt")
}

func uniqFile(filename string, out io.Writer, showCount, showRepeated, showUnique, ignoreCase bool, checkChars, skipFields int) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return uniqReader(file, out, showCount, showRepeated, showUnique, ignoreCase, checkChars, skipFields)
}

func uniqReader(r io.Reader, out io.Writer, showCount, showRepeated, showUnique, ignoreCase bool, checkChars, skipFields int) error {
	scanner := bufio.NewScanner(r)

	var prevOrigLine string // Original previous line for output
	var prevNormLine string // Normalized previous line for comparison
	var prevCount int
	var isFirst bool = true

	normalizeLine := func(line string) string {
		result := line

		// Skip fields if requested
		if skipFields > 0 {
			fields := strings.Fields(result)
			if len(fields) > skipFields {
				result = strings.Join(fields[skipFields:], " ")
			} else {
				result = ""
			}
		}

		// Check chars if requested
		if checkChars > 0 && len(result) > checkChars {
			result = result[:checkChars]
		}

		// Ignore case if requested
		if ignoreCase {
			result = strings.ToLower(result)
		}

		return result
	}

	for scanner.Scan() {
		line := scanner.Text()
		normalized := normalizeLine(line)

		if isFirst {
			prevOrigLine = line
			prevNormLine = normalized
			prevCount = 1
			isFirst = false
			continue
		}

		if normalized == prevNormLine {
			prevCount++
		} else {
			// Process the previous group
			shouldPrint := true
			if showRepeated {
				shouldPrint = prevCount > 1
			} else if showUnique {
				shouldPrint = prevCount == 1
			}

			if shouldPrint {
				if showCount {
					fmt.Fprintf(out, "%7d %s\n", prevCount, prevOrigLine)
				} else {
					fmt.Fprintln(out, prevOrigLine)
				}
			}

			prevOrigLine = line
			prevNormLine = normalized
			prevCount = 1
		}
	}

	// Process the last group
	if !isFirst {
		shouldPrint := true
		if showRepeated {
			shouldPrint = prevCount > 1
		} else if showUnique {
			shouldPrint = prevCount == 1
		}

		if shouldPrint {
			if showCount {
				fmt.Fprintf(out, "%7d %s\n", prevCount, prevOrigLine)
			} else {
				fmt.Fprintln(out, prevOrigLine)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
