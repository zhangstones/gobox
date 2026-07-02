package text

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExitCodeError wraps a shell-style exit code for commands such as grep -q.
type ExitCodeError int
type exitCodeError = ExitCodeError

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

var errExitQuiet = ExitCodeError(1)

type grepOptions struct {
	ignoreCase        bool
	invert            bool
	count             bool
	lineNumber        bool
	recursive         bool
	showFilename      bool
	fixedString       bool
	onlyMatching      bool
	quiet             bool
	lineBuffered      bool
	filesWithMatches  bool
	filesWithoutMatch bool
	beforeContext     int
	afterContext      int
	includePattern    string
	excludeDir        string
}

type grepResult struct {
	matches int
	matched bool
}

// GrepCmd implements a basic subset of grep functionality.
func GrepCmd(args []string) error {
	fsFlags := flag.NewFlagSet("grep", flag.ContinueOnError)
	ignoreCase := fsFlags.Bool("i", false, "ignore case")
	invert := fsFlags.Bool("v", false, "invert match (show non-matching lines)")
	count := fsFlags.Bool("c", false, "show count of matching lines only")
	lineNumber := fsFlags.Bool("n", false, "show line numbers")
	recursive := fsFlags.Bool("r", false, "recursive search in directories")
	fixedString := fsFlags.Bool("F", false, "interpret pattern as fixed string (not regex)")
	_ = fsFlags.Bool("E", false, "use extended regular expressions (ERE) - default in Go")
	onlyMatching := fsFlags.Bool("o", false, "print only the matched parts of a line")
	quiet := fsFlags.Bool("q", false, "suppress all normal output (exit code only)")
	lineBuffered := fsFlags.Bool("line-buffered", false, "use line buffering (flush after each line)")
	afterContext := fsFlags.Int("A", 0, "print NUM lines of trailing context")
	beforeContext := fsFlags.Int("B", 0, "print NUM lines of leading context")
	context := fsFlags.Int("C", 0, "print NUM lines of output context")
	afterContextLong := fsFlags.Int("after-context", 0, "print NUM lines of trailing context")
	beforeContextLong := fsFlags.Int("before-context", 0, "print NUM lines of leading context")
	contextLong := fsFlags.Int("context", 0, "print NUM lines of output context")
	includePattern := fsFlags.String("include", "", "search only files that match PATTERN")
	excludeDir := fsFlags.String("exclude-dir", "", "skip directories named DIR")
	filesWithMatches := fsFlags.Bool("l", false, "print only names of files with selected lines")
	filesWithoutMatch := fsFlags.Bool("L", false, "print only names of files without selected lines")
	filesWithMatchesLong := fsFlags.Bool("files-with-matches", false, "print only names of files with selected lines")
	filesWithoutMatchLong := fsFlags.Bool("files-without-match", false, "print only names of files without selected lines")
	help := fsFlags.Bool("help", false, "show help")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox grep [OPTION]... PATTERN [FILE...]")
		fmt.Fprintln(os.Stderr, "Search for PATTERN in each FILE or standard input.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Matching:")
		fmt.Fprintln(os.Stderr, "  -i                      ignore case")
		fmt.Fprintln(os.Stderr, "  -v                      invert match")
		fmt.Fprintln(os.Stderr, "  -F                      interpret pattern as fixed string")
		fmt.Fprintln(os.Stderr, "  -E                      use extended regular expressions (default in Go)")
		fmt.Fprintln(os.Stderr, "  -o                      print only matching text")
		fmt.Fprintln(os.Stderr, "  -q                      suppress normal output and return status only")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -c                      show count of matching lines only")
		fmt.Fprintln(os.Stderr, "  -n                      show line numbers")
		fmt.Fprintln(os.Stderr, "  --line-buffered         flush output after each line")
		fmt.Fprintln(os.Stderr, "  -l, --files-with-matches")
		fmt.Fprintln(os.Stderr, "                          print only names of files with selected lines")
		fmt.Fprintln(os.Stderr, "  -L, --files-without-match")
		fmt.Fprintln(os.Stderr, "                          print only names of files without selected lines")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Context:")
		fmt.Fprintln(os.Stderr, "  -r                      recursive search in directories")
		fmt.Fprintln(os.Stderr, "  -A, --after-context N   print N trailing context lines")
		fmt.Fprintln(os.Stderr, "  -B, --before-context N  print N leading context lines")
		fmt.Fprintln(os.Stderr, "  -C, --context N         print N lines of surrounding context")
		fmt.Fprintln(os.Stderr, "  --include PATTERN       search only files matching PATTERN")
		fmt.Fprintln(os.Stderr, "  --exclude-dir DIR       skip directories named DIR")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox grep \"error\" /var/log/syslog")
		fmt.Fprintln(os.Stderr, "  gobox grep -i -r \"TODO\" /path/to/code")
		fmt.Fprintln(os.Stderr, "  gobox grep -v \"^#\" config.txt")
		fmt.Fprintln(os.Stderr, "  gobox grep -o \"[0-9]+\" file.txt  # only matching parts")
		fmt.Fprintln(os.Stderr, "  gobox grep -q \"pattern\" file && echo \"found\"")
		fmt.Fprintln(os.Stderr, "  cat file.txt | gobox grep \"pattern\"")
	}

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *help {
		fsFlags.Usage()
		return nil
	}
	if fsFlags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "grep: PATTERN is required")
		fsFlags.Usage()
		return fmt.Errorf("pattern required")
	}
	if *afterContextLong > 0 {
		*afterContext = *afterContextLong
	}
	if *beforeContextLong > 0 {
		*beforeContext = *beforeContextLong
	}
	if *contextLong > 0 {
		*context = *contextLong
	}
	if *filesWithMatchesLong {
		*filesWithMatches = true
	}
	if *filesWithoutMatchLong {
		*filesWithoutMatch = true
	}
	if *filesWithMatches && *filesWithoutMatch {
		return fmt.Errorf("-l and -L cannot be used together")
	}

	pattern := fsFlags.Arg(0)
	files := fsFlags.Args()[1:]

	opts := grepOptions{
		ignoreCase:        *ignoreCase,
		invert:            *invert,
		count:             *count,
		lineNumber:        *lineNumber,
		recursive:         *recursive,
		showFilename:      len(files) > 1 || *recursive,
		fixedString:       *fixedString,
		onlyMatching:      *onlyMatching,
		quiet:             *quiet,
		lineBuffered:      *lineBuffered,
		filesWithMatches:  *filesWithMatches,
		filesWithoutMatch: *filesWithoutMatch,
		beforeContext:     *beforeContext,
		afterContext:      *afterContext,
		includePattern:    *includePattern,
		excludeDir:        *excludeDir,
	}
	if *context > 0 {
		if opts.beforeContext == 0 {
			opts.beforeContext = *context
		}
		if opts.afterContext == 0 {
			opts.afterContext = *context
		}
	}

	var regex *regexp.Regexp
	if !opts.fixedString {
		var err error
		if opts.ignoreCase {
			regex, err = regexp.Compile("(?i)" + pattern)
		} else {
			regex, err = regexp.Compile(pattern)
		}
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	} else if opts.ignoreCase {
		var err error
		regex, err = regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
		if err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
	}

	matchedAny := false
	if len(files) == 0 {
		result, err := grepReader(os.Stdin, pattern, regex, opts, "")
		if err != nil {
			return err
		}
		matchedAny = result.matched
	} else {
		for _, file := range files {
			var err error
			if opts.recursive {
				err = filepath.WalkDir(file, func(path string, d os.DirEntry, walkErr error) error {
					if walkErr != nil {
						return nil
					}
					if d.IsDir() {
						if opts.excludeDir != "" && d.Name() == opts.excludeDir {
							return filepath.SkipDir
						}
						return nil
					}
					if opts.includePattern != "" {
						matched, matchErr := filepath.Match(opts.includePattern, d.Name())
						if matchErr != nil || !matched {
							return nil
						}
					}
					result, err := grepFile(path, pattern, regex, opts)
					if err != nil {
						return err
					}
					matchedAny = matchedAny || result.matched
					if opts.quiet && matchedAny {
						return errExitQuiet
					}
					return nil
				})
				if err == errExitQuiet {
					err = nil
				}
			} else {
				result, grepErr := grepFile(file, pattern, regex, opts)
				if grepErr != nil {
					return grepErr
				}
				matchedAny = matchedAny || result.matched
				if opts.quiet && matchedAny {
					break
				}
			}
			if err != nil {
				return err
			}
		}
	}

	if opts.quiet && !matchedAny {
		return errExitQuiet
	}
	return nil
}

func grepFile(path, pattern string, regex *regexp.Regexp, opts grepOptions) (grepResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return grepResult{}, fmt.Errorf("cannot open %s: %w", path, err)
	}
	defer file.Close()
	return grepReader(file, pattern, regex, opts, path)
}

func grepReader(r io.Reader, pattern string, regex *regexp.Regexp, opts grepOptions, filename string) (grepResult, error) {
	// When context or filesWithoutMatch requires seeing all lines, buffer first.
	if opts.beforeContext > 0 || opts.filesWithoutMatch {
		scanner := bufio.NewScanner(r)
		var lines []string
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			name := filename
			if name == "" {
				name = "stdin"
			}
			return grepResult{}, fmt.Errorf("error reading %s: %w", name, err)
		}
		return grepLines(lines, pattern, regex, opts, filename)
	}
	return grepReaderStream(r, pattern, regex, opts, filename)
}

// grepReaderStream processes lines one at a time when no before-context buffering is needed.
func grepReaderStream(r io.Reader, pattern string, regex *regexp.Regexp, opts grepOptions, filename string) (grepResult, error) {
	scanner := bufio.NewScanner(r)
	matches := 0
	lineNum := 0
	afterRemain := 0 // lines still to print from afterContext window

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		matched := grepLineMatches(line, pattern, regex, opts)
		if matched {
			matches++
			if opts.quiet {
				return grepResult{matches: matches, matched: true}, nil
			}
			if opts.filesWithMatches {
				fmt.Fprintln(os.Stdout, filename)
				return grepResult{matches: matches, matched: true}, nil
			}
			afterRemain = opts.afterContext
		}
		if matched || afterRemain > 0 {
			if !opts.count && !opts.quiet && !opts.filesWithMatches {
				if opts.onlyMatching && matched {
					for _, part := range grepFindMatches(line, pattern, regex, opts) {
						printGrepLineWithOptions(part, filename, lineNum, opts)
					}
				} else if !opts.onlyMatching {
					printGrepLineWithOptions(line, filename, lineNum, opts)
				}
			}
			if !matched && afterRemain > 0 {
				afterRemain--
			}
		}
	}
	if err := scanner.Err(); err != nil {
		name := filename
		if name == "" {
			name = "stdin"
		}
		return grepResult{}, fmt.Errorf("error reading %s: %w", name, err)
	}
	matched := matches > 0
	if opts.count && !opts.quiet {
		if opts.showFilename && filename != "" {
			fmt.Printf("%s:%d\n", filename, matches)
		} else {
			fmt.Println(matches)
		}
	}
	return grepResult{matches: matches, matched: matched}, nil
}

func grepLines(lines []string, pattern string, regex *regexp.Regexp, opts grepOptions, filename string) (grepResult, error) {
	matchedLines := make([]bool, len(lines))
	matches := 0
	for i, line := range lines {
		matched := grepLineMatches(line, pattern, regex, opts)
		if matched {
			matchedLines[i] = true
			matches++
			if opts.quiet {
				return grepResult{matches: matches, matched: true}, nil
			}
		}
	}
	matched := matches > 0

	if opts.filesWithMatches {
		if matched {
			fmt.Fprintln(os.Stdout, filename)
		}
		return grepResult{matches: matches, matched: matched}, nil
	}
	if opts.filesWithoutMatch {
		if !matched {
			fmt.Fprintln(os.Stdout, filename)
		}
		return grepResult{matches: matches, matched: matched}, nil
	}
	if opts.count && !opts.quiet {
		if opts.showFilename && filename != "" {
			fmt.Printf("%s:%d\n", filename, matches)
		} else {
			fmt.Println(matches)
		}
		return grepResult{matches: matches, matched: matched}, nil
	}
	if opts.quiet || !matched {
		return grepResult{matches: matches, matched: matched}, nil
	}

	if opts.onlyMatching {
		for i, line := range lines {
			if !matchedLines[i] {
				continue
			}
			for _, part := range grepFindMatches(line, pattern, regex, opts) {
				printGrepLineWithOptions(part, filename, i+1, opts)
			}
		}
		return grepResult{matches: matches, matched: matched}, nil
	}

	toPrint := make([]bool, len(lines))
	for i, ok := range matchedLines {
		if !ok {
			continue
		}
		start := i - opts.beforeContext
		if start < 0 {
			start = 0
		}
		end := i + opts.afterContext
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for j := start; j <= end; j++ {
			toPrint[j] = true
		}
	}
	for i, line := range lines {
		if toPrint[i] {
			printGrepLineWithOptions(line, filename, i+1, opts)
		}
	}
	return grepResult{matches: matches, matched: matched}, nil
}

func grepLineMatches(line, pattern string, regex *regexp.Regexp, opts grepOptions) bool {
	var matched bool
	if opts.fixedString {
		if opts.ignoreCase {
			matched = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
		} else {
			matched = strings.Contains(line, pattern)
		}
	} else {
		matched = regex.MatchString(line)
	}
	if opts.invert {
		matched = !matched
	}
	return matched
}

func grepFindMatches(line, pattern string, regex *regexp.Regexp, opts grepOptions) []string {
	if opts.invert {
		return nil
	}
	if opts.fixedString && !opts.ignoreCase {
		parts := make([]string, 0)
		start := 0
		for {
			idx := strings.Index(line[start:], pattern)
			if idx == -1 {
				break
			}
			actualIdx := start + idx
			parts = append(parts, line[actualIdx:actualIdx+len(pattern)])
			start = actualIdx + len(pattern)
		}
		return parts
	}
	return regex.FindAllString(line, -1)
}

func printGrepLineWithOptions(line, filename string, lineNum int, opts grepOptions) {
	if opts.showFilename && filename != "" {
		fmt.Fprintf(os.Stdout, "%s:", filename)
	}
	if opts.lineNumber {
		fmt.Fprintf(os.Stdout, "%d:", lineNum)
	}
	fmt.Fprintln(os.Stdout, line)
}
