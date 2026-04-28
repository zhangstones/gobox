package fs

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FindCmd implements a basic subset of busybox find
func FindCmd(args []string) error {
	args = normalizeFindArgs(args)
	fsFlags := flag.NewFlagSet("find", flag.ContinueOnError)
	name := fsFlags.String("name", "", "match basename with pattern (shell glob)")
	pathPattern := fsFlags.String("path", "", "match full path with pattern (shell glob)")
	negate := fsFlags.Bool("not", false, "negate the combined match result")
	typ := fsFlags.String("type", "", "file type: f (file) or d (dir)")
	maxdepth := fsFlags.Int("maxdepth", -1, "maximum depth")
	mindepth := fsFlags.Int("mindepth", 0, "minimum depth")
	printFlag := fsFlags.Bool("print", true, "print matched paths")
	empty := fsFlags.Bool("empty", false, "match empty files or directories")
	size := fsFlags.String("size", "", "file size: +N (larger than N), -N (smaller than N), N (equal to N) (K/M/G suffixes supported)")
	atime := fsFlags.String("atime", "", "file access time: +N, -N, N (N[smh] = seconds/minutes/hours/days)")
	mtime := fsFlags.String("mtime", "", "file modify time: +N, -N, N (N[smh] = seconds/minutes/hours/days)")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox find [OPTION]... [PATH...]")
		fmt.Fprintln(os.Stderr, "Search for files in a directory hierarchy.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Filters:")
		fmt.Fprintln(os.Stderr, "  -name PATTERN      match basename with shell glob")
		fmt.Fprintln(os.Stderr, "  -path PATTERN      match full path with shell glob")
		fmt.Fprintln(os.Stderr, "  -type TYPE         file type: f (file) or d (directory)")
		fmt.Fprintln(os.Stderr, "  -empty             match empty files or directories")
		fmt.Fprintln(os.Stderr, "  -size SPEC         size filter: +N, -N, N with optional K/M/G suffix")
		fmt.Fprintln(os.Stderr, "  -atime SPEC        access time filter: +N, -N, N with optional s/m/h suffix")
		fmt.Fprintln(os.Stderr, "  -mtime SPEC        modify time filter: +N, -N, N with optional s/m/h suffix")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Traversal:")
		fmt.Fprintln(os.Stderr, "  -maxdepth N        descend at most N levels")
		fmt.Fprintln(os.Stderr, "  -mindepth N        skip matches shallower than N levels")
		fmt.Fprintln(os.Stderr, "  -not               negate the combined match result")
		fmt.Fprintln(os.Stderr, "  -print             print matched paths (default true)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox find . -type f -name '*.log'")
		fmt.Fprintln(os.Stderr, "  gobox find /tmp -maxdepth 2 -empty")
	}

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *typ != "" && *typ != "f" && *typ != "d" {
		return fmt.Errorf("invalid type %q: must be 'f' or 'd'", *typ)
	}

	if fsFlags.NArg() > 0 {
		for _, arg := range fsFlags.Args() {
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unexpected option %q found after flags", arg)
			}
		}
	}

	paths := fsFlags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Debug output
	if os.Getenv("DEBUG_FIND") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: paths=%v, name='%s', path='%s', typ='%s', not=%v, maxdepth=%d, mindepth=%d, empty=%v, size='%s', atime='%s', mtime='%s'\n",
			paths, *name, *pathPattern, *typ, *negate, *maxdepth, *mindepth, *empty, *size, *atime, *mtime)
	}

	for _, root := range paths {
		root = filepath.Clean(root)
		baseDepth := pathDepth(root)
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				// Continue on permission errors
				return nil
			}
			depth := pathDepth(p) - baseDepth
			if *maxdepth >= 0 && depth > *maxdepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if depth < *mindepth {
				return nil
			}

			matched := true

			// type filter
			if *typ != "" {
				if *typ == "f" && d.IsDir() {
					matched = false
				}
				if *typ == "d" && !d.IsDir() {
					matched = false
				}
			}

			// name filter (glob pattern matching, not regex)
			if matched && *name != "" {
				// Use filepath.Match for glob pattern matching
				// Patterns: * matches any sequence, ? matches any single char, [abc] matches char class
				nameMatched, err := filepath.Match(*name, d.Name())
				if err != nil || !nameMatched {
					matched = false
				}
			}

			if matched && *pathPattern != "" {
				matched = matchPathPattern(*pathPattern, p)
			}

			// empty filter
			if matched && *empty {
				if d.IsDir() {
					// Check if directory is empty
					entries, err := os.ReadDir(p)
					if err != nil {
						matched = false
					}
					if matched && len(entries) > 0 {
						matched = false
					}
				} else {
					// Check if file is empty
					info, err := d.Info()
					if err != nil {
						matched = false
					}
					if matched && info.Size() > 0 {
						matched = false
					}
				}
			}

			// size filter
			if matched && *size != "" {
				if d.IsDir() {
					// Match GNU find semantics: size filtering applies to files here, so directories do not match.
					matched = false
				} else {
					info, err := d.Info()
					if err != nil {
						matched = false
					}
					if matched {
						fileSize := info.Size()
						if !matchSize(fileSize, *size) {
							matched = false
						}
					}
				}
			}

			// atime filter (access time)
			if matched && *atime != "" {
				info, err := d.Info()
				if err != nil {
					matched = false
				}
				if matched && !matchTime(info, *atime, "atime") {
					matched = false
				}
			}

			// mtime filter (modify time)
			if matched && *mtime != "" {
				info, err := d.Info()
				if err != nil {
					matched = false
				}
				if matched && !matchTime(info, *mtime, "mtime") {
					matched = false
				}
			}

			if *negate {
				matched = !matched
			}

			if matched && *printFlag {
				fmt.Println(p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func normalizeFindArgs(args []string) []string {
	normalized := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "!" {
			normalized = append(normalized, "-not")
			continue
		}
		normalized = append(normalized, arg)
	}
	return normalized
}

func matchPathPattern(pattern, path string) bool {
	pattern = filepath.ToSlash(filepath.Clean(pattern))
	candidate := filepath.ToSlash(filepath.Clean(path))
	re, err := regexp.Compile(globToRegex(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(candidate)
}

func globToRegex(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', '[', ']', '\\':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		default:
			b.WriteByte(pattern[i])
		}
	}
	b.WriteString("$")
	return b.String()
}

func pathDepth(p string) int {
	if p == "." || p == "" || p == string(filepath.Separator) {
		return 0
	}
	p = filepath.Clean(p)
	return len(strings.Split(p, string(filepath.Separator)))
}

// parseSize parses size specification with optional prefix and suffix
// Format: [+|-]N[K|M|G|T]
// +N: larger than N
// -N: smaller than N
// N: equal to N (or if just number, larger than or equal)
// Suffixes: K (1024), M (1024*1024), G (1024*1024*1024), T (1024*1024*1024*1024)
func parseSize(sizeSpec string) (int64, int, error) {
	if sizeSpec == "" {
		return 0, 0, fmt.Errorf("size specification is empty")
	}

	operator := 0 // 0 = equal, 1 = greater than, -1 = less than
	spec := sizeSpec

	if strings.HasPrefix(spec, "+") {
		operator = 1
		spec = spec[1:]
	} else if strings.HasPrefix(spec, "-") {
		operator = -1
		spec = spec[1:]
	}

	// Parse numeric part and suffix
	multiplier := int64(1)
	var numPart string

	// Check for size suffixes
	if strings.HasSuffix(spec, "K") || strings.HasSuffix(spec, "k") {
		multiplier = 1024
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "M") || strings.HasSuffix(spec, "m") {
		multiplier = 1024 * 1024
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "G") || strings.HasSuffix(spec, "g") {
		multiplier = 1024 * 1024 * 1024
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "T") || strings.HasSuffix(spec, "t") {
		multiplier = 1024 * 1024 * 1024 * 1024
		numPart = spec[:len(spec)-1]
	} else {
		numPart = spec
	}

	num, err := strconv.ParseInt(numPart, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid size value: %s", sizeSpec)
	}

	return num * multiplier, operator, nil
}

// matchSize checks if a file size matches the given size specification
func matchSize(fileSize int64, sizeSpec string) bool {
	targetSize, operator, err := parseSize(sizeSpec)
	if err != nil {
		return false
	}

	switch operator {
	case 1: // larger than
		return fileSize > targetSize
	case -1: // smaller than
		return fileSize < targetSize
	case 0: // equal to
		return fileSize == targetSize
	default:
		return false
	}
}

// parseTime parses time specification with optional prefix and unit
// Format: [+|-]N[s|m|h|d]
// +N: older than N (more than N time units ago)
// -N: newer than N (less than N time units ago)
// N: exactly N time units ago (within [N, N+1) units)
// Units: s (seconds), m (minutes), h (hours), d (days, default)
func parseTime(timeSpec string) (time.Duration, int, error) {
	if timeSpec == "" {
		return 0, 0, fmt.Errorf("time specification is empty")
	}

	operator := 0 // 0 = exact, 1 = older (more than), -1 = newer (less than)
	spec := timeSpec

	if strings.HasPrefix(spec, "+") {
		operator = 1
		spec = spec[1:]
	} else if strings.HasPrefix(spec, "-") {
		operator = -1
		spec = spec[1:]
	}

	// Parse numeric part and unit
	unit := time.Hour * 24 // default: days
	var numPart string

	// Check for time unit suffixes
	if strings.HasSuffix(spec, "s") {
		unit = time.Second
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "m") {
		unit = time.Minute
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "h") {
		unit = time.Hour
		numPart = spec[:len(spec)-1]
	} else if strings.HasSuffix(spec, "d") {
		unit = time.Hour * 24
		numPart = spec[:len(spec)-1]
	} else {
		// No suffix, assume days
		numPart = spec
	}

	num, err := strconv.ParseInt(numPart, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time value: %s", timeSpec)
	}

	return time.Duration(num) * unit, operator, nil
}

// matchTime checks if a file's access or modify time matches the given time specification
func matchTime(info fs.FileInfo, timeSpec string, timeType string) bool {
	targetDuration, operator, err := parseTime(timeSpec)
	if err != nil {
		return false
	}

	var fileTime time.Time
	if timeType == "atime" {
		// Get access time from platform-specific stat info
		stat := info.Sys()
		if stat != nil {
			// On Unix systems, extract atime from stat structure
			if st, ok := stat.(*syscall.Stat_t); ok {
				fileTime = time.Unix(st.Atim.Sec, st.Atim.Nsec)
			} else {
				// Fallback to mtime if unable to extract atime
				fileTime = info.ModTime()
			}
		} else {
			return false
		}
	} else if timeType == "mtime" {
		fileTime = info.ModTime()
	} else {
		return false
	}

	now := time.Now()
	timeSinceFileTime := now.Sub(fileTime)

	switch operator {
	case 1: // older than (more than N units ago)
		return timeSinceFileTime > targetDuration
	case -1: // newer than (less than N units ago)
		return timeSinceFileTime < targetDuration
	case 0: // exactly N
		tolerance := 24 * time.Hour
		spec := strings.TrimLeft(timeSpec, "+-")
		switch {
		case strings.HasSuffix(spec, "s"):
			tolerance = time.Second
		case strings.HasSuffix(spec, "m"):
			tolerance = time.Minute
		case strings.HasSuffix(spec, "h"):
			tolerance = time.Hour
		}
		return timeSinceFileTime >= targetDuration && timeSinceFileTime < targetDuration+tolerance
	default:
		return false
	}
}
