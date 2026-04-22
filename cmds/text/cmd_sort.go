package text

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type sortConfig struct {
	numeric        bool
	reverse        bool
	key            int    // 1-based column number, 0 = whole line
	sep            string // field separator
	unique         bool
	month          bool
	human          bool
	random         bool
	check          bool
	output         string
	zeroTerminated bool
}

type sortExitError struct {
	code int
	err  error
}

func (e sortExitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}

func (e sortExitError) Unwrap() error {
	return e.err
}

func (e sortExitError) ExitCode() int {
	return e.code
}

var monthNames = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March,
	"apr": time.April, "may": time.May, "jun": time.June,
	"jul": time.July, "aug": time.August, "sep": time.September,
	"oct": time.October, "nov": time.November, "dec": time.December,
}

func SortCmd(args []string) error {
	cfg := sortConfig{key: 0}

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--numeric-sort":
			cfg.numeric = true
		case arg == "-r" || arg == "--reverse":
			cfg.reverse = true
		case arg == "-u" || arg == "--unique":
			cfg.unique = true
		case arg == "-M" || arg == "--month-sort":
			cfg.month = true
		case arg == "-h" || arg == "--human-numeric-sort":
			cfg.human = true
		case arg == "-R" || arg == "--random-sort":
			cfg.random = true
		case arg == "-c" || arg == "--check":
			cfg.check = true
		case arg == "-z" || arg == "--zero-terminated":
			cfg.zeroTerminated = true
		case arg == "-k":
			if i+1 >= len(args) {
				return fmt.Errorf("-k requires an argument")
			}
			i++
			key, err := strconv.Atoi(args[i])
			if err != nil || key < 1 {
				return fmt.Errorf("invalid key number: %s", args[i])
			}
			cfg.key = key
		case strings.HasPrefix(arg, "-k"):
			keyStr := arg[2:]
			key, err := strconv.Atoi(keyStr)
			if err != nil || key < 1 {
				return fmt.Errorf("invalid key number: %s", keyStr)
			}
			cfg.key = key
		case arg == "--key=":
			return fmt.Errorf("--key= requires an argument")
		case strings.HasPrefix(arg, "--key="):
			keyStr := arg[6:]
			key, err := strconv.Atoi(keyStr)
			if err != nil || key < 1 {
				return fmt.Errorf("invalid key number: %s", keyStr)
			}
			cfg.key = key
		case arg == "-t":
			if i+1 >= len(args) {
				return fmt.Errorf("-t requires an argument")
			}
			i++
			cfg.sep = args[i]
		case arg == "--field-separator=":
			cfg.sep = arg[18:]
		case arg == "-o":
			if i+1 >= len(args) {
				return fmt.Errorf("-o requires an argument")
			}
			i++
			cfg.output = args[i]
		case arg == "--output=":
			cfg.output = arg[9:]
		case arg == "-h" || arg == "--help":
			printSortUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--"):
			// Handle combined flags like -ru
			for j := 1; j < len(arg); j++ {
				switch arg[j] {
				case 'n':
					cfg.numeric = true
				case 'r':
					cfg.reverse = true
				case 'u':
					cfg.unique = true
				case 'M':
					cfg.month = true
				case 'h':
					cfg.human = true
				case 'R':
					cfg.random = true
				case 'c':
					cfg.check = true
				case 'z':
					cfg.zeroTerminated = true
				default:
					return fmt.Errorf("unknown option: -%c", arg[j])
				}
			}
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			goto doneFlags
		}
		i++
	}

doneFlags:
	files := args[i:]

	// Read input
	var lines []string
	if len(files) == 0 {
		lines = readLines(os.Stdin, cfg.zeroTerminated)
	} else {
		for _, file := range files {
			f, err := os.Open(file)
			if err != nil {
				return fmt.Errorf("cannot open %s: %w", file, err)
			}
			lines = append(lines, readLines(f, cfg.zeroTerminated)...)
			f.Close()
		}
	}

	// Check mode
	if cfg.check {
		return checkSorted(lines, cfg)
	}

	// Sort
	sorted, err := sortLines(lines, cfg)
	if err != nil {
		return err
	}

	// Output
	var out io.Writer = os.Stdout
	if cfg.output != "" {
		f, err := os.Create(cfg.output)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		out = f
		defer f.Close()
	}

	writeLines(out, sorted, cfg.zeroTerminated)
	return nil
}

func readLines(r io.Reader, zeroTerminated bool) []string {
	var lines []string
	if zeroTerminated {
		scanner := bufio.NewScanner(r)
		scanner.Split(scanZeroTerminated)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	} else {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
	}
	return lines
}

func scanZeroTerminated(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func writeLines(w io.Writer, lines []string, zeroTerminated bool) {
	for _, line := range lines {
		if zeroTerminated {
			fmt.Fprintf(w, "%s\x00", line)
		} else {
			fmt.Fprintln(w, line)
		}
	}
}

func getField(line string, key int, sep string) string {
	if key == 0 {
		return line
	}
	if sep == "" {
		fields := strings.Fields(line)
		if key <= len(fields) {
			return fields[key-1]
		}
		return ""
	}
	parts := strings.Split(line, sep)
	if key <= len(parts) {
		return parts[key-1]
	}
	return ""
}

func parseValue(field string, cfg sortConfig) interface{} {
	if cfg.month {
		lower := strings.ToLower(field)
		if month, ok := monthNames[lower]; ok {
			return month
		}
		// Return January (1) for invalid months - this ensures consistent type
		// and sorts invalid months before valid ones (since 0 < 1 < 2 < ...)
		return time.January
	}
	if cfg.human {
		return parseHumanNumber(field)
	}
	if cfg.numeric {
		f, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return 0.0
		}
		return f
	}
	return field
}

var humanRegex = regexp.MustCompile(`^([0-9.]+)([KMGT]?)([i]?)?$`)

func parseHumanNumber(s string) float64 {
	s = strings.TrimSpace(s)
	m := humanRegex.FindStringSubmatch(s)
	if m == nil {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	num, _ := strconv.ParseFloat(m[1], 64)
	unit := m[2]
	switch unit {
	case "K":
		num *= 1024
	case "M":
		num *= 1024 * 1024
	case "G":
		num *= 1024 * 1024 * 1024
	case "T":
		num *= 1024 * 1024 * 1024 * 1024
	}
	return num
}

type sortEntry struct {
	line     string
	value    interface{}
	original int
}

func sortLines(lines []string, cfg sortConfig) ([]string, error) {
	if cfg.random {
		// Fisher-Yates shuffle with thread-safe rng
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		result := make([]string, len(lines))
		indices := rng.Perm(len(lines))
		for i, idx := range indices {
			result[i] = lines[idx]
		}
		return result, nil
	}

	entries := make([]sortEntry, len(lines))
	for i, line := range lines {
		field := getField(line, cfg.key, cfg.sep)
		entries[i] = sortEntry{
			line:     line,
			value:    parseValue(field, cfg),
			original: i,
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		vi, vj := entries[i].value, entries[j].value

		// Compare based on type
		var less bool
		switch v := vi.(type) {
		case float64:
			vjF := vj.(float64)
			less = v < vjF
		case time.Month:
			vjM := vj.(time.Month)
			less = v < vjM
		case int:
			vjI := vj.(int)
			less = v < vjI
		default:
			vs := vi.(string)
			vsj := vj.(string)
			less = strings.Compare(vs, vsj) < 0
		}

		if cfg.reverse {
			return !less
		}
		return less
	})

	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.line
	}

	if cfg.unique {
		result = uniqueLines(result)
	}

	return result, nil
}

func uniqueLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	var result []string
	seen := ""
	for _, line := range lines {
		if line != seen {
			result = append(result, line)
			seen = line
		}
	}
	return result
}

func checkSorted(lines []string, cfg sortConfig) error {
	if len(lines) == 0 {
		fmt.Fprintf(os.Stdout, "Sort: %s: succeeded\n", "(empty)")
		return nil
	}
	if len(lines) == 1 {
		fmt.Fprintf(os.Stdout, "Sort: %s: succeeded\n", lines[0])
		return nil
	}

	entries := make([]sortEntry, len(lines))
	for i, line := range lines {
		field := getField(line, cfg.key, cfg.sep)
		entries[i] = sortEntry{
			line:     line,
			value:    parseValue(field, cfg),
			original: i,
		}
	}

	for i := 1; i < len(entries); i++ {
		vi, vj := entries[i-1].value, entries[i].value
		var less bool
		switch v := vi.(type) {
		case float64:
			vjF := vj.(float64)
			less = v >= vjF
		case time.Month:
			vjM := vj.(time.Month)
			less = v >= vjM
		case int:
			vjI := vj.(int)
			less = v >= vjI
		default:
			vs := vi.(string)
			vsj := vj.(string)
			less = strings.Compare(vs, vsj) >= 0
		}
		if cfg.reverse {
			less = !less
		}
		if less {
			fmt.Fprintf(os.Stderr, "Sort: %s: disorder: line %d\n", os.Stdin.Name(), i+1)
			return sortExitError{code: 1, err: errors.New("check failed")}
		}
	}
	fmt.Fprintln(os.Stdout, "sorted")
	return nil
}

func printSortUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox sort [OPTIONS] [FILE]")
	fmt.Fprintln(w, "Sort lines of text files.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n, --numeric-sort       Sort by numeric value")
	fmt.Fprintln(w, "  -r, --reverse            Reverse order")
	fmt.Fprintln(w, "  -k, --key=NUM            Sort by column NUM")
	fmt.Fprintln(w, "  -t, --field-separator=CHAR   Use CHAR as field separator")
	fmt.Fprintln(w, "  -u, --unique            Remove duplicate lines")
	fmt.Fprintln(w, "  -M, --month-sort        Sort by month")
	fmt.Fprintln(w, "  -h, --human-numeric-sort   Sort by human readable numbers (1K, 2M)")
	fmt.Fprintln(w, "  -R, --random-sort       Random sort")
	fmt.Fprintln(w, "  -c, --check             Check if sorted")
	fmt.Fprintln(w, "  -o, --output=FILE       Write to FILE")
	fmt.Fprintln(w, "  -z, --zero-terminated   Lines end with 0 byte")
	fmt.Fprintln(w, "  -h, --help              Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox sort file.txt")
	fmt.Fprintln(w, "  gobox sort -n file.txt")
	fmt.Fprintln(w, "  gobox sort -k2 -t: /etc/passwd")
	fmt.Fprintln(w, "  gobox sort -ru file.txt")
	fmt.Fprintln(w, "  cat file.txt | gobox sort")
}
