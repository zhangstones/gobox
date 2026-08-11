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

	"gobox/cmds/utils"
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
			if i+1 >= len(args) || utils.LooksLikeFlag(args[i+1]) {
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
			if i+1 >= len(args) || utils.LooksLikeFlag(args[i+1]) {
				return fmt.Errorf("-t requires an argument")
			}
			i++
			cfg.sep = args[i]
		case strings.HasPrefix(arg, "-t"):
			cfg.sep = arg[2:]
		case arg == "--field-separator=":
			return fmt.Errorf("--field-separator= requires an argument")
		case strings.HasPrefix(arg, "--field-separator="):
			cfg.sep = arg[len("--field-separator="):]
		case arg == "-o":
			if i+1 >= len(args) || utils.LooksLikeFlag(args[i+1]) {
				return fmt.Errorf("-o requires an argument")
			}
			i++
			cfg.output = args[i]
		case arg == "--output=":
			return fmt.Errorf("--output= requires an argument")
		case strings.HasPrefix(arg, "--output="):
			cfg.output = arg[len("--output="):]
		case arg == "--help":
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
		sourceName := "-"
		if len(files) > 0 {
			sourceName = files[0]
		}
		return checkSorted(lines, cfg, sourceName)
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
		// GNU sort -M matches only the leading month abbreviation and ignores
		// any trailing content on the line (e.g. "Jan 5", "Mar 15 12:00 ..."),
		// rather than requiring the whole field to equal a month name.
		token := field
		if fields := strings.Fields(field); len(fields) > 0 {
			token = fields[0]
		}
		lower := strings.ToLower(token)
		if len(lower) > 3 {
			lower = lower[:3]
		}
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

	sort.SliceStable(entries, func(i, j int) bool {
		cmp := compareSortValues(entries[i].value, entries[j].value)
		if cfg.reverse {
			return cmp > 0
		}
		return cmp < 0
	})

	// -u removes entries whose SORT KEY compares equal to the previous kept
	// entry (GNU sort semantics), not just byte-identical whole lines. After a
	// stable sort, equal keys are adjacent, so a single pass suffices.
	if cfg.unique {
		deduped := entries[:0]
		for _, e := range entries {
			if len(deduped) == 0 || compareSortValues(deduped[len(deduped)-1].value, e.value) != 0 {
				deduped = append(deduped, e)
			}
		}
		entries = deduped
	}

	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = e.line
	}

	return result, nil
}

// compareSortValues returns -1, 0, or 1 comparing two parsed sort values.
// Within a single sort run both values always share the same dynamic type
// (all float64, all time.Month, or all string), matching parseValue.
func compareSortValues(vi, vj interface{}) int {
	switch v := vi.(type) {
	case float64:
		vjF := vj.(float64)
		if v < vjF {
			return -1
		} else if v > vjF {
			return 1
		}
		return 0
	case time.Month:
		vjM := vj.(time.Month)
		if v < vjM {
			return -1
		} else if v > vjM {
			return 1
		}
		return 0
	case int:
		vjI := vj.(int)
		if v < vjI {
			return -1
		} else if v > vjI {
			return 1
		}
		return 0
	default:
		return strings.Compare(vi.(string), vj.(string))
	}
}

func checkSorted(lines []string, cfg sortConfig, sourceName string) error {
	if len(lines) <= 1 {
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
		// cmp: -1 if prev<cur, 0 if equal, 1 if prev>cur.
		cmp := compareSortValues(entries[i-1].value, entries[i].value)
		// Equal adjacent lines are in order; only a strict inversion is disorder.
		disorder := cmp > 0
		if cfg.reverse {
			disorder = cmp < 0
		}
		if disorder {
			fmt.Fprintf(os.Stderr, "sort: %s: disorder: line %d\n", sourceName, i+1)
			return sortExitError{code: 1, err: errors.New("check failed")}
		}
	}
	return nil
}

func printSortUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox sort [OPTION]... [FILE]")
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
	fmt.Fprintln(w, "      --help              Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox sort file.txt")
	fmt.Fprintln(w, "  gobox sort -n file.txt")
	fmt.Fprintln(w, "  gobox sort -k2 -t: /etc/passwd")
	fmt.Fprintln(w, "  gobox sort -ru file.txt")
	fmt.Fprintln(w, "  cat file.txt | gobox sort")
}
