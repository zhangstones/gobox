package disk

import (
	"bufio"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var md5HexPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

type md5sumExitError struct {
	code int
	err  error
}

func (e md5sumExitError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return fmt.Sprintf("exit code %d", e.code)
}

func (e md5sumExitError) Unwrap() error {
	return e.err
}

func (e md5sumExitError) ExitCode() int {
	return e.code
}

func Md5sumCmd(args []string) error {
	fsFlags := flag.NewFlagSet("md5sum", flag.ContinueOnError)
	var checkMode bool
	var tag bool
	var quiet bool
	var status bool
	var warn bool
	fsFlags.BoolVar(&checkMode, "c", false, "check MD5 sums against provided file")
	fsFlags.BoolVar(&checkMode, "check", false, "check MD5 sums against provided file")
	fsFlags.BoolVar(&tag, "tag", false, "produce BSD style output (MD5 (file) = xxx)")
	fsFlags.BoolVar(&quiet, "q", false, "quiet mode")
	fsFlags.BoolVar(&quiet, "quiet", false, "quiet mode")
	fsFlags.BoolVar(&status, "s", false, "only return status code")
	fsFlags.BoolVar(&status, "status", false, "only return status code")
	fsFlags.BoolVar(&warn, "w", false, "warn about malformed lines")
	fsFlags.BoolVar(&warn, "warn", false, "warn about malformed lines")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox md5sum [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Compute or check MD5 message digests.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Modes:")
		fmt.Fprintln(os.Stderr, "  -c, --check      check MD5 sums from files")
		fmt.Fprintln(os.Stderr, "      --tag        use BSD style output (MD5 (file) = xxx)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Output:")
		fmt.Fprintln(os.Stderr, "  -q, --quiet      quiet mode")
		fmt.Fprintln(os.Stderr, "  -s, --status     status mode")
		fmt.Fprintln(os.Stderr, "  -w, --warn       warn about malformed lines")
		fmt.Fprintln(os.Stderr, "  -h, --help       show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	files := fsFlags.Args()

	// If no files and no stdin, show usage
	if len(files) == 0 {
		stat, _ := os.Stdin.Stat()
		hasStdinData := (stat.Mode() & os.ModeCharDevice) == 0
		if checkMode {
			// -c with no file operands reads the checksum list from stdin,
			// matching GNU md5sum -- it must not fall through to compute
			// mode and hash the stdin bytes themselves.
			if hasStdinData {
				return md5sumCheckStdin(warn, status, quiet)
			}
			fsFlags.Usage()
			return errors.New("no files specified")
		}
		if hasStdinData {
			// Data is available on stdin
			if err := md5sumStdin(tag, quiet); err != nil {
				return err
			}
			return nil
		}
		fsFlags.Usage()
		return errors.New("no files specified")
	}

	if checkMode {
		return md5sumCheck(files, warn, status, quiet)
	}

	if quiet {
		return md5sumQuietError{}
	}

	// Default: compute mode
	return md5sumFiles(files, tag, quiet)
}

type md5sumQuietError struct{}

func (md5sumQuietError) Error() string {
	return "the --quiet option is meaningful only when verifying checksums"
}
func (md5sumQuietError) ExitCode() int          { return 1 }
func (md5sumQuietError) SuppressCLIError() bool { return false }

func md5sumStdin(tag, quiet bool) error {
	h := md5.New()
	if _, err := io.Copy(h, os.Stdin); err != nil {
		return err
	}
	hashStr := fmt.Sprintf("%x", h.Sum(nil))
	if tag {
		fmt.Printf("MD5 (stdin) = %s\n", hashStr)
	} else if quiet {
		fmt.Println(hashStr)
	} else {
		fmt.Printf("%s  -\n", hashStr)
	}
	return nil
}

func md5sumFiles(files []string, tag, quiet bool) error {
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", file, err)
			}
			continue
		}
		hash, err := computeMD5(f)
		f.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", file, err)
			}
			continue
		}
		hashStr := fmt.Sprintf("%x", hash)
		if tag {
			fmt.Printf("MD5 (%s) = %s\n", file, hashStr)
		} else if quiet {
			fmt.Println(hashStr)
		} else {
			fmt.Printf("%s  %s\n", hashStr, file)
		}
	}
	return nil
}

func computeMD5(r io.Reader) ([]byte, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func md5sumCheck(files []string, warn, status, quiet bool) error {
	var hasError bool

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", file, err)
			}
			hasError = true
			continue
		}
		if md5sumCheckReader(f, file, warn, status, quiet) {
			hasError = true
		}
		f.Close()
	}

	if hasError {
		return md5sumExitError{code: 1, err: errors.New("checksum verification failed")}
	}
	return nil
}

// md5sumCheckStdin implements `-c` with no file operands: GNU md5sum reads
// the checksum list from stdin in that case (rather than falling through to
// compute mode and hashing the stdin bytes themselves).
func md5sumCheckStdin(warn, status, quiet bool) error {
	if md5sumCheckReader(os.Stdin, "-", warn, status, quiet) {
		return md5sumExitError{code: 1, err: errors.New("checksum verification failed")}
	}
	return nil
}

// md5sumCheckReader reads a checksum list (POSIX "<hash>  <filename>" or BSD
// "MD5 (filename) = hash" format) from r and verifies each referenced file,
// reporting via sourceName in diagnostics. Returns true if any line failed
// to parse or any referenced file's checksum did not match.
func md5sumCheckReader(r io.Reader, sourceName string, warn, status, quiet bool) bool {
	var hasError bool
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if line == "" {
			continue
		}

		// Parse the line: expected format is "<hash> <filename>" or "MD5 (file) = hash"
		var expectedHash, filename string

		// Try BSD format: MD5 (file) = hash
		if strings.HasPrefix(line, "MD5 (") {
			// Format: MD5 (filename) = hash
			parts := strings.SplitN(line, " = ", 2)
			if len(parts) != 2 {
				if warn {
					fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted BSD style checksum line\n", sourceName, lineNum)
				}
				hasError = true
				continue
			}
			// Extract filename from "MD5 (filename)"
			middle := strings.TrimPrefix(parts[0], "MD5 (")
			if !strings.HasSuffix(middle, ")") {
				if warn {
					fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted BSD style checksum line\n", sourceName, lineNum)
				}
				hasError = true
				continue
			}
			filename = strings.TrimSuffix(middle, ")")
			expectedHash = parts[1]
		} else {
			// Try POSIX format: hash  filename (two spaces between)
			// Some implementations use "  " as separator, some use " *"
			parts := strings.Fields(line)
			if len(parts) < 2 {
				if warn {
					fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted checksum line\n", sourceName, lineNum)
				}
				hasError = true
				continue
			}
			expectedHash = parts[0]
			if !md5HexPattern.MatchString(expectedHash) {
				if warn {
					fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted checksum line\n", sourceName, lineNum)
				}
				hasError = true
				continue
			}
			// Filename might have spaces, so join remaining parts; strip binary-mode prefix
			filename = strings.TrimPrefix(strings.Join(parts[1:], " "), "*")
		}

		// Compute actual hash
		fileToCheck, err := os.Open(filename)
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", filename, err)
			}
			if !quiet && !status {
				fmt.Printf("%s: FAILED open or read\n", filename)
			}
			hasError = true
			continue
		}

		hash, err := computeMD5(fileToCheck)
		fileToCheck.Close()
		if err != nil {
			if !quiet {
				fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", filename, err)
			}
			hasError = true
			continue
		}

		actualHash := fmt.Sprintf("%x", hash)

		if !status {
			if actualHash == expectedHash {
				if !quiet {
					fmt.Printf("%s: OK\n", filename)
				}
			} else {
				fmt.Printf("%s: FAILED\n", filename)
				hasError = true
			}
		} else {
			if actualHash != expectedHash {
				hasError = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "md5sum: %s: error reading: %v\n", sourceName, err)
		}
		hasError = true
	}
	return hasError
}
