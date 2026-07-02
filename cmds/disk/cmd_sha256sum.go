package disk

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var sha256HexPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

type sha256sumExitError struct {
	code int
	err  error
}

func (e sha256sumExitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}
func (e sha256sumExitError) ExitCode() int { return e.code }

func Sha256sumCmd(args []string) error {
	fsFlags := flag.NewFlagSet("sha256sum", flag.ContinueOnError)
	checkMode := fsFlags.Bool("c", false, "check SHA256 sums")
	fsFlags.BoolVar(checkMode, "check", false, "check SHA256 sums")
	tag := fsFlags.Bool("tag", false, "BSD style output")
	quiet := fsFlags.Bool("q", false, "quiet mode")
	fsFlags.BoolVar(quiet, "quiet", false, "quiet mode")
	status := fsFlags.Bool("s", false, "status mode")
	fsFlags.BoolVar(status, "status", false, "status mode")
	warn := fsFlags.Bool("w", false, "warn malformed lines")
	fsFlags.BoolVar(warn, "warn", false, "warn malformed lines")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox sha256sum [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Compute or check SHA256 message digests.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Modes:")
		fmt.Fprintln(os.Stderr, "  -c, --check       check SHA256 sums from files")
		fmt.Fprintln(os.Stderr, "  --tag             use BSD style output")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -q, --quiet       quiet mode")
		fmt.Fprintln(os.Stderr, "  -s, --status      status mode")
		fmt.Fprintln(os.Stderr, "  -w, --warn        warn about malformed lines")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	files := fsFlags.Args()
	if *checkMode {
		if len(files) == 0 {
			return fmt.Errorf("sha256sum: check mode requires a file")
		}
		return sha256sumCheck(files, *warn, *status, *quiet)
	}
	if len(files) == 0 {
		sum, err := computeSHA256(os.Stdin)
		if err != nil {
			return err
		}
		printSHA256("-", sum, *tag, *quiet)
		return nil
	}
	var hadErr bool
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			hadErr = true
			if !*quiet {
				fmt.Fprintf(os.Stderr, "sha256sum: %s: %v\n", file, err)
			}
			continue
		}
		sum, err := computeSHA256(f)
		_ = f.Close()
		if err != nil {
			hadErr = true
			if !*quiet {
				fmt.Fprintf(os.Stderr, "sha256sum: %s: %v\n", file, err)
			}
			continue
		}
		printSHA256(file, sum, *tag, *quiet)
	}
	if hadErr {
		return sha256sumExitError{code: 1}
	}
	return nil
}

func computeSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func printSHA256(file, sum string, tag, quiet bool) {
	if tag {
		name := file
		if name == "-" {
			name = "stdin"
		}
		fmt.Printf("SHA256 (%s) = %s\n", name, sum)
	} else if quiet {
		fmt.Println(sum)
	} else {
		fmt.Printf("%s  %s\n", sum, file)
	}
}

func sha256sumCheck(files []string, warn, status, quiet bool) error {
	var failed bool
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			failed = true
			if !status && !quiet {
				fmt.Fprintf(os.Stderr, "sha256sum: %s: %v\n", file, err)
			}
			continue
		}
		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			expected, name, ok := parseSHA256CheckLine(line)
			if !ok {
				failed = true
				if warn && !status && !quiet {
					fmt.Fprintf(os.Stderr, "sha256sum: %s:%d: improperly formatted checksum line\n", file, lineNo)
				}
				continue
			}
			actual, err := sha256File(name)
			if err != nil {
				failed = true
				if !status && !quiet {
					fmt.Fprintf(os.Stderr, "sha256sum: %s: %v\n", name, err)
					fmt.Printf("%s: FAILED\n", name)
				}
			} else if !strings.EqualFold(actual, expected) {
				failed = true
				if !status && !quiet {
					fmt.Printf("%s: FAILED\n", name)
				}
			} else if !status && !quiet {
				fmt.Printf("%s: OK\n", name)
			}
		}
		_ = f.Close()
		if err := scanner.Err(); err != nil {
			return err
		}
	}
	if failed {
		return sha256sumExitError{code: 1}
	}
	return nil
}

func parseSHA256CheckLine(line string) (string, string, bool) {
	if strings.HasPrefix(line, "SHA256 (") {
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) != 2 {
			return "", "", false
		}
		name := strings.TrimSuffix(strings.TrimPrefix(parts[0], "SHA256 ("), ")")
		return parts[1], name, sha256HexPattern.MatchString(parts[1])
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || !sha256HexPattern.MatchString(fields[0]) {
		return "", "", false
	}
	return fields[0], strings.TrimPrefix(strings.Join(fields[1:], " "), "*"), true
}

func sha256File(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return computeSHA256(f)
}
