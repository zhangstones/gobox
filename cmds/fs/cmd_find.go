package fs

import (
	"flag"
	"fmt"
	"gobox/cmds/utils"
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
	// GNU find binds -not/! to the predicate that immediately follows it, not
	// to the whole expression. Pull those per-predicate negations out before
	// flag parsing; any remaining bare -not keeps the legacy whole-result flag.
	negated, args := extractNegations(args)
	fsFlags := flag.NewFlagSet("find", flag.ContinueOnError)
	name := fsFlags.String("name", "", "match basename with pattern (shell glob)")
	pathPattern := fsFlags.String("path", "", "match full path with pattern (shell glob)")
	negate := fsFlags.Bool("not", false, "negate the following predicate (legacy: whole result if unbound)")
	typ := fsFlags.String("type", "", "file type: f (file) or d (dir)")
	maxdepth := fsFlags.Int("maxdepth", -1, "maximum depth")
	mindepth := fsFlags.Int("mindepth", 0, "minimum depth")
	printFlag := fsFlags.Bool("print", true, "print matched paths")
	empty := fsFlags.Bool("empty", false, "match empty files or directories")
	size := fsFlags.String("size", "", "file size: +N (larger than N), -N (smaller than N), N (equal to N) (c/K/M/G suffixes; default unit is bytes)")
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
		fmt.Fprintln(os.Stderr, "  -size SPEC         size filter: +N, -N, N with optional c/K/M/G suffix (default bytes)")
		fmt.Fprintln(os.Stderr, "  -atime SPEC        access time filter: +N, -N, N with optional s/m/h suffix")
		fmt.Fprintln(os.Stderr, "  -mtime SPEC        modify time filter: +N, -N, N with optional s/m/h suffix")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Traversal:")
		fmt.Fprintln(os.Stderr, "  -maxdepth N        descend at most N levels")
		fmt.Fprintln(os.Stderr, "  -mindepth N        skip matches shallower than N levels")
		fmt.Fprintln(os.Stderr, "  -not, !            negate the immediately following predicate")
		fmt.Fprintln(os.Stderr, "  -print             print matched paths (default true)")
		fmt.Fprintln(os.Stderr, "  -h, --help         show this help")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox find . -type f -name '*.log'")
		fmt.Fprintln(os.Stderr, "  gobox find /tmp -maxdepth 2 -empty")
	}

	flagArgs, pathArgs := splitFindArgs(args)
	if err := utils.ParseFlagSet(fsFlags, flagArgs); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *typ != "" && *typ != "f" && *typ != "d" {
		return fmt.Errorf("invalid type %q: must be 'f' or 'd'", *typ)
	}

	// fsFlags.Args() should be empty since splitFindArgs already separated
	// out the paths, but guard against any stray flag-shaped leftovers.
	if fsFlags.NArg() > 0 {
		for _, arg := range fsFlags.Args() {
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unexpected option %q found after flags", arg)
			}
		}
	}

	paths := append(fsFlags.Args(), pathArgs...)
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Debug output
	if os.Getenv("DEBUG_FIND") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: paths=%v, name='%s', path='%s', typ='%s', not=%v, maxdepth=%d, mindepth=%d, empty=%v, size='%s', atime='%s', mtime='%s'\n",
			paths, *name, *pathPattern, *typ, *negate, *maxdepth, *mindepth, *empty, *size, *atime, *mtime)
	}

	// Compile path pattern regex once before walking.
	var pathPatternRe *regexp.Regexp
	if *pathPattern != "" {
		cleanPat := filepath.ToSlash(filepath.Clean(*pathPattern))
		if re, err := regexp.Compile(globToRegex(cleanPat)); err == nil {
			pathPatternRe = re
		}
	}

	for _, root := range paths {
		// Preserve the root exactly as given for display (GNU find prints the
		// starting point verbatim, e.g. "find ." yields "./x"), but walk the
		// cleaned path since WalkDir requires a real filesystem path.
		origRoot := root
		cleanRoot := filepath.Clean(root)
		baseDepth := pathDepth(cleanRoot)
		err := filepath.WalkDir(cleanRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == cleanRoot {
					return err
				}
				return nil
			}
			// display is the path as GNU find would print it (root prefix preserved).
			display := origRoot
			if rel, relErr := filepath.Rel(cleanRoot, p); relErr == nil && rel != "." {
				display = joinDisplayPath(origRoot, rel)
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

			// Each predicate is evaluated to a raw boolean, negated on its own
			// if it was preceded by -not/!, then AND-combined into matched.
			matched := true
			apply := func(pred string, ok bool) {
				if negated[pred] {
					ok = !ok
				}
				matched = matched && ok
			}

			// type filter
			if *typ != "" {
				ok := true
				if *typ == "f" && !d.Type().IsRegular() {
					ok = false
				}
				if *typ == "d" && !d.IsDir() {
					ok = false
				}
				apply("type", ok)
			}

			// name filter (glob pattern matching, not regex)
			if *name != "" {
				// Use filepath.Match for glob pattern matching
				// Patterns: * matches any sequence, ? matches any single char, [abc] matches char class
				nameMatched, err := filepath.Match(*name, d.Name())
				apply("name", err == nil && nameMatched)
			}

			if pathPatternRe != nil {
				// GNU find matches -path against the printed path, which keeps
				// the root prefix (so "find . -path '*/x/*'" can match "./x/...").
				candidate := filepath.ToSlash(display)
				apply("path", pathPatternRe.MatchString(candidate))
			}

			// empty filter
			if *empty {
				ok := true
				if d.IsDir() {
					// Check if directory is empty
					entries, err := os.ReadDir(p)
					if err != nil || len(entries) > 0 {
						ok = false
					}
				} else {
					// Check if file is empty
					info, err := d.Info()
					if err != nil || info.Size() > 0 {
						ok = false
					}
				}
				apply("empty", ok)
			}

			// size filter
			if *size != "" {
				ok := true
				if d.IsDir() {
					// Match GNU find semantics: size filtering applies to files here, so directories do not match.
					ok = false
				} else {
					info, err := d.Info()
					if err != nil || !matchSize(info.Size(), *size) {
						ok = false
					}
				}
				apply("size", ok)
			}

			// atime filter (access time)
			if *atime != "" {
				info, err := d.Info()
				apply("atime", err == nil && matchTime(info, *atime, "atime"))
			}

			// mtime filter (modify time)
			if *mtime != "" {
				info, err := d.Info()
				apply("mtime", err == nil && matchTime(info, *mtime, "mtime"))
			}

			if *negate {
				matched = !matched
			}

			if matched && *printFlag {
				fmt.Println(display)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// findBoolFlags lists find's flags that take no argument value, so that
// splitFindArgs knows not to consume the next token as a flag value.
var findBoolFlags = map[string]bool{
	"not":   true,
	"print": true,
	"empty": true,
}

// splitFindArgs separates flag tokens (and their values) from bare path
// arguments regardless of where they appear on the command line. This lets
// paths be given before, after, or interleaved with flags (e.g.
// `find . -type f -name '*.log'`), matching GNU find and gobox's own
// documented usage example, instead of requiring all flags to precede paths.
func splitFindArgs(args []string) (flagArgs []string, paths []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			paths = append(paths, args[i+1:]...)
			break
		}
		if len(arg) > 1 && arg[0] == '-' {
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				// value embedded via -name=value, nothing more to consume
				continue
			}
			if !findBoolFlags[name] && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		paths = append(paths, arg)
	}
	return flagArgs, paths
}

// findPredicateFlags lists the match predicates that a preceding -not/! can
// negate individually (GNU find semantics).
var findPredicateFlags = map[string]bool{
	"name":  true,
	"path":  true,
	"type":  true,
	"empty": true,
	"size":  true,
	"atime": true,
	"mtime": true,
}

// extractNegations consumes -not tokens that immediately precede a match
// predicate and records them as per-predicate negations, returning the
// remaining args with those -not tokens removed. Consecutive -not tokens
// toggle (double negation cancels). A -not that does not bind to a predicate
// is left in place so the FlagSet still applies the legacy whole-result flag.
func extractNegations(args []string) (map[string]bool, []string) {
	negated := make(map[string]bool)
	out := make([]string, 0, len(args))
	pendingNot := false
	for _, arg := range args {
		if arg == "-not" {
			pendingNot = !pendingNot
			continue
		}
		if pendingNot && len(arg) > 1 && arg[0] == '-' {
			pred := strings.TrimLeft(arg, "-")
			if eq := strings.IndexByte(pred, '='); eq >= 0 {
				pred = pred[:eq]
			}
			if findPredicateFlags[pred] {
				negated[pred] = !negated[pred]
				pendingNot = false
				out = append(out, arg)
				continue
			}
		}
		if pendingNot {
			// Unbound -not: preserve legacy whole-result negation.
			out = append(out, "-not")
			pendingNot = false
		}
		out = append(out, arg)
	}
	if pendingNot {
		out = append(out, "-not")
	}
	return negated, out
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

// joinDisplayPath joins a root prefix with a relative path without cleaning,
// so a "." root is preserved as "./rel" the way GNU find prints it.
func joinDisplayPath(root, rel string) string {
	if root == "" {
		return rel
	}
	if strings.HasSuffix(root, string(filepath.Separator)) {
		return root + rel
	}
	return root + string(filepath.Separator) + rel
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
		case '[':
			// pass bracket expression through verbatim
			j := i + 1
			if j < len(pattern) && pattern[j] == '!' {
				j++
			}
			if j < len(pattern) && pattern[j] == ']' {
				j++
			}
			for j < len(pattern) && pattern[j] != ']' {
				j++
			}
			if j < len(pattern) {
				b.WriteString(pattern[i : j+1])
				i = j
			} else {
				b.WriteString(`\[`)
			}
		case '.', '+', '(', ')', '|', '^', '$', '{', '}', ']', '\\':
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
	p = filepath.Clean(p)
	if p == "." {
		return 0
	}
	count := 0
	for _, s := range strings.Split(p, string(filepath.Separator)) {
		if s != "" {
			count++
		}
	}
	return count
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
	} else if strings.HasSuffix(spec, "c") || strings.HasSuffix(spec, "C") {
		// GNU find: 'c' means bytes (the default unit in gobox is already bytes).
		multiplier = 1
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
