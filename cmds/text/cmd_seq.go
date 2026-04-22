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
		case strings.HasPrefix(arg, "-f") && !strings.HasPrefix(arg, "--"):
			format = arg[2:]
		case strings.HasPrefix(arg, "--format="):
			format = arg[9:]
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
				return fmt.Errorf("unknown option: %s", arg)
			}
			goto doneFlags
		}
		i++
	}

doneFlags:
	remaining := args[i:]

	switch len(remaining) {
	case 0:
		return fmt.Errorf("missing operand")
	case 1:
		first = 1
		inc = 1
		var err error
		last, err = strconv.ParseFloat(remaining[0], 64)
		if err != nil {
			return fmt.Errorf("invalid number: %s", remaining[0])
		}
	case 2:
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
		width = len(fmt.Sprintf("%.0f", absMax))
		if minVal < 0 {
			width++
		}
	}

	cur := first
	if inc > 0 {
		for cur <= last {
			if width > 0 {
				fmtStr := "%0" + strconv.Itoa(width) + "g"
				fmt.Printf(fmtStr, cur)
			} else {
				fmt.Printf(format, cur)
			}
			cur += inc
			if cur <= last {
				fmt.Printf("%s", separator)
			}
		}
	} else {
		for cur >= last {
			if width > 0 {
				fmtStr := "%0" + strconv.Itoa(width) + "g"
				fmt.Printf(fmtStr, cur)
			} else {
				fmt.Printf(format, cur)
			}
			cur += inc
			if cur >= last {
				fmt.Printf("%s", separator)
			}
		}
	}
	fmt.Println()
	return nil
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
