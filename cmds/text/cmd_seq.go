package text

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func SeqCmd(args []string) error {
	var first, inc, last float64
	var format string
	var separator string
	var equalWidth bool
	var formatExplicit bool

	format = "%g"
	separator = "\n"

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-f" || arg == "--format":
			if i+1 >= len(args) {
				return fmt.Errorf("-f requires an argument")
			}
			i++
			format = args[i]
			formatExplicit = true
		case strings.HasPrefix(arg, "-f") && !strings.HasPrefix(arg, "--"):
			format = arg[2:]
			formatExplicit = true
		case strings.HasPrefix(arg, "--format="):
			format = arg[9:]
			formatExplicit = true
		case arg == "-s" || arg == "--separator":
			if i+1 >= len(args) {
				return fmt.Errorf("-s requires an argument")
			}
			i++
			separator = args[i]
		case strings.HasPrefix(arg, "-s") && !strings.HasPrefix(arg, "--"):
			separator = arg[2:]
		case strings.HasPrefix(arg, "--separator="):
			separator = arg[12:]
		case arg == "-w" || arg == "--equal-width":
			equalWidth = true
		case arg == "-h" || arg == "--help":
			printSeqUsage(os.Stdout)
			return nil
		default:
			if strings.HasPrefix(arg, "-") {
				// A dash-prefixed token that parses as a number is a
				// negative FIRST/INC/LAST operand (e.g. "seq -5 5"), not an
				// unrecognized flag.
				if _, err := strconv.ParseFloat(arg, 64); err != nil {
					return fmt.Errorf("unknown option: %s", arg)
				}
			}
			goto doneFlags
		}
		i++
	}

doneFlags:
	remaining := args[i:]

	var firstStr, incStr, lastStr string
	switch len(remaining) {
	case 0:
		return fmt.Errorf("missing operand")
	case 1:
		first, inc = 1, 1
		firstStr, incStr = "1", "1"
		lastStr = remaining[0]
		var err error
		last, err = strconv.ParseFloat(remaining[0], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[0])
		}
	case 2:
		firstStr, incStr, lastStr = remaining[0], "1", remaining[1]
		var err error
		first, err = strconv.ParseFloat(remaining[0], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[0])
		}
		inc = 1
		last, err = strconv.ParseFloat(remaining[1], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[1])
		}
	case 3:
		firstStr, incStr, lastStr = remaining[0], remaining[1], remaining[2]
		var err error
		first, err = strconv.ParseFloat(remaining[0], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[0])
		}
		inc, err = strconv.ParseFloat(remaining[1], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[1])
		}
		if inc == 0 {
			return fmt.Errorf("invalid increment: 0")
		}
		last, err = strconv.ParseFloat(remaining[2], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[2])
		}
	default:
		return fmt.Errorf("too many arguments")
	}

	if inc == 0 {
		return fmt.Errorf("invalid increment: 0")
	}

	// GNU seq derives the output decimal precision from the operands' own
	// string representations (not the parsed float64 values), and applies
	// it consistently to every generated value. Besides matching native
	// output, this also avoids floating-point accumulation artifacts (e.g.
	// summing 0.1 three times rendering as "0.30000000000000004").
	decimals := decimalPlaces(firstStr)
	if d := decimalPlaces(incStr); d > decimals {
		decimals = d
	}
	if d := decimalPlaces(lastStr); d > decimals {
		decimals = d
	}

	width := 0
	if equalWidth {
		maxVal := last
		minVal := first
		if last < first {
			maxVal = first
			minVal = last
		}
		absMax := maxVal
		if absMax < 0 {
			absMax = -absMax
		}
		width = len(strconv.FormatFloat(absMax, 'f', 0, 64))
		if minVal < 0 {
			width++
		}
	}

	// Compute each value from first+n*inc (not by repeatedly adding inc to
	// a running total) to avoid compounding floating-point error over long
	// sequences.
	for n := 0; ; n++ {
		cur := first + float64(n)*inc
		if inc > 0 && cur > last {
			break
		}
		if inc < 0 && cur < last {
			break
		}
		if n > 0 {
			fmt.Print(separator)
		}
		switch {
		case width > 0:
			fmt.Print(formatSeqEqualWidth(cur, width, decimals))
		case formatExplicit:
			fmt.Printf(format, cur)
		default:
			fmt.Print(strconv.FormatFloat(cur, 'f', decimals, 64))
		}
	}
	fmt.Println()
	return nil
}

// decimalPlaces returns the number of digits after the decimal point in a
// numeric string operand (0 if there is no decimal point).
func decimalPlaces(s string) int {
	idx := strings.IndexByte(s, '.')
	if idx < 0 {
		return 0
	}
	return len(s) - idx - 1
}

// formatSeqEqualWidth renders v with the given decimal precision, then
// zero-pads its integer part (not the decimal portion) up to width
// characters (width already accounts for a leading '-' sign, matching the
// -w/--equal-width computation above).
func formatSeqEqualWidth(v float64, width, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, fracPart := s, ""
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		intPart, fracPart = s[:idx], s[idx:]
	}
	intWidth := width
	if neg {
		intWidth--
	}
	if pad := intWidth - len(intPart); pad > 0 {
		intPart = strings.Repeat("0", pad) + intPart
	}
	if neg {
		return "-" + intPart + fracPart
	}
	return intPart + fracPart
}

func printSeqUsage(w *os.File) {
	fmt.Fprintln(w, "Usage: gobox seq [OPTIONS] [FIRST [INC]] LAST")
	fmt.Fprintln(w, "Print sequences of numbers.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintf(w, "  -f, --format=FORMAT   Use printf-style FORMAT (default %%g)\n")
	fmt.Fprintln(w, "  -s, --separator=SEP  Use SEP to separate numbers (default \\n)")
	fmt.Fprintln(w, "  -w, --equal-width    Equalize width by padding with leading zeros")
	fmt.Fprintln(w, "  -h, --help           Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox seq 5                    # 1 2 3 4 5")
	fmt.Fprintln(w, "  gobox seq 2 5                  # 2 3 4 5")
	fmt.Fprintln(w, "  gobox seq 0 2 10               # 0 2 4 6 8 10")
	fmt.Fprintln(w, "  gobox seq -f \"%02g\" 5           # 01 02 03 04 05")
	fmt.Fprintln(w, "  gobox seq -s \",\" 3              # 1,2,3")
	fmt.Fprintln(w, "  gobox seq -w 9                 # 01 02 ... 09")
}
