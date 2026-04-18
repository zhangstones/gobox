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

// exitCodeError wraps an exit code for grep's quiet mode
type exitCodeError int

func (e exitCodeError) Error() string {
	return fmt.Sprintf("exit code %d", int(e))
}

var errExitQuiet = exitCodeError(1)

// GrepCmd implements a basic subset of grep functionality
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
	help := fsFlags.Bool("help", false, "show help")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox grep [OPTIONS] PATTERN [FILE...]")
		fmt.Fprintln(os.Stderr, "Search for PATTERN in each FILE or standard input.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fsFlags.PrintDefaults()
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

	pattern := fsFlags.Arg(0)
	files := fsFlags.Args()[1:]

	// Compile regex or use fixed string matching
	var regex *regexp.Regexp
	if !*fixedString {
		var err error
		if *ignoreCase {
			regex, err = regexp.Compile("(?i)" + pattern)
		} else {
			regex, err = regexp.Compile(pattern)
		}
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	// If no files specified, read from stdin
	if len(files) == 0 {
		if err := grepReader(os.Stdin, pattern, regex, *ignoreCase, *invert, *count, *lineNumber, *fixedString, *onlyMatching, *quiet, *lineBuffered, ""); err != nil {
			return err
		}
		return nil
	}

	// Process files
	totalMatches := 0
	for _, file := range files {
		if *recursive {
			err := filepath.WalkDir(file, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil // Skip errors, continue
				}
				if d.IsDir() {
					return nil
				}
				return grepFile(path, pattern, regex, *ignoreCase, *invert, *count, *lineNumber, *fixedString, *onlyMatching, *quiet, *lineBuffered, &totalMatches)
			})
			if err != nil {
				return err
			}
		} else {
			if err := grepFile(file, pattern, regex, *ignoreCase, *invert, *count, *lineNumber, *fixedString, *onlyMatching, *quiet, *lineBuffered, &totalMatches); err != nil {
				return err
			}
		}
	}

	return nil
}

func grepFile(path, pattern string, regex *regexp.Regexp, ignoreCase, invert, count, lineNumber, fixedString, onlyMatching, quiet, lineBuffered bool, totalMatches *int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", path, err)
	}
	defer file.Close()

	return grepReader(file, pattern, regex, ignoreCase, invert, count, lineNumber, fixedString, onlyMatching, quiet, lineBuffered, path)
}

func grepReader(r io.Reader, pattern string, regex *regexp.Regexp, ignoreCase, invert, count, lineNumber, fixedString, onlyMatching, quiet, lineBuffered bool, filename string) error {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	matches := 0
	var out io.Writer = os.Stdout

	// Enable line buffering if requested
	if lineBuffered {
		out = os.Stdout // Go's stdout is already line-buffered when connected to terminal
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var matched bool
		if fixedString {
			// Fixed string matching
			if ignoreCase {
				matched = strings.Contains(strings.ToLower(line), strings.ToLower(pattern))
			} else {
				matched = strings.Contains(line, pattern)
			}
		} else {
			// Regex matching
			matched = regex.MatchString(line)
		}

		// Invert match if -v is specified
		if invert {
			matched = !matched
		}

		if matched {
			matches++
			if !quiet && !count {
				if onlyMatching {
					// Print only matched parts
					if fixedString {
						// Fixed string matching with -o: find all occurrences
						searchPattern := pattern
						searchLine := line
						if ignoreCase {
							searchPattern = strings.ToLower(pattern)
							searchLine = strings.ToLower(line)
						}
						
						start := 0
						for {
							idx := strings.Index(searchLine[start:], searchPattern)
							if idx == -1 {
								break
							}
							actualIdx := start + idx
							matchedStr := line[actualIdx : actualIdx+len(pattern)]
							if filename != "" {
								fmt.Fprintf(out, "%s:", filename)
							}
							if lineNumber {
								fmt.Fprintf(out, "%d:", lineNum)
							}
							fmt.Fprintln(out, matchedStr)
							start = actualIdx + len(pattern)
						}
					} else {
						// Regex matching with -o
						var re *regexp.Regexp
						if ignoreCase {
							// Re-compile with case sensitivity for FindAllString
							var err error
							re, err = regexp.Compile(pattern)
							if err != nil {
								re = regex
							}
						} else {
							re = regex
						}
						
						foundMatches := re.FindAllString(line, -1)
						for _, m := range foundMatches {
							if filename != "" {
								fmt.Fprintf(out, "%s:", filename)
							}
							if lineNumber {
								fmt.Fprintf(out, "%d:", lineNum)
							}
							fmt.Fprintln(out, m)
						}
					}
				} else {
					// Print entire line
					if filename != "" {
						fmt.Fprintf(out, "%s:", filename)
					}
					if lineNumber {
						fmt.Fprintf(out, "%d:", lineNum)
					}
					fmt.Fprintln(out, line)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading %s: %w", filename, err)
	}

	if count && !quiet {
		if filename != "" {
			fmt.Printf("%s:%d\n", filename, matches)
		} else {
			fmt.Println(matches)
		}
	}

	// Exit with code 1 if no matches found and not quiet
	if quiet && matches == 0 {
		return errExitQuiet
	}

	return nil
}
