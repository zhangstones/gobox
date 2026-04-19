package disk

import (
	"bufio"
	"crypto/md5"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func Md5sumCmd(args []string) error {
	fsFlags := flag.NewFlagSet("md5sum", flag.ContinueOnError)
	checkMode := fsFlags.Bool("c", false, "check MD5 sums against provided file")
	tag := fsFlags.Bool("tag", false, "produce BSD style output (MD5 (file) = xxx)")
	quiet := fsFlags.Bool("q", false, "quiet mode")
	status := fsFlags.Bool("s", false, "only return status code")
	warn := fsFlags.Bool("w", false, "warn about malformed lines")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox md5sum [OPTION]... [FILE]...")
		fmt.Fprintln(os.Stderr, "Compute or check MD5 checksums.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -c, --check      verify checksums against file")
		fmt.Fprintln(os.Stderr, "      --tag        BSD style output (MD5 (file) = xxx)")
		fmt.Fprintln(os.Stderr, "  -q, --quiet      quiet mode")
		fmt.Fprintln(os.Stderr, "  -s, --status     only return status code")
		fmt.Fprintln(os.Stderr, "  -w, --warn       warn about malformed lines")
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
		// Check if there's data on stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is available on stdin
			if err := md5sumStdin(*tag, *quiet); err != nil {
				return err
			}
			return nil
		}
		fsFlags.Usage()
		return errors.New("no files specified")
	}

	if *checkMode {
		return md5sumCheck(files, *warn, *status, *quiet)
	}

	// Default: compute mode
	return md5sumFiles(files, *tag, *quiet)
}

func md5sumStdin(tag, quiet bool) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	hash := md5.Sum(data)
	hashStr := fmt.Sprintf("%x", hash)
	if tag {
		fmt.Printf("MD5 (stdin) = %s\n", hashStr)
	} else {
		fmt.Printf("%s  -\n", hashStr)
	}
	_ = quiet
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
		defer f.Close()

		scanner := bufio.NewScanner(f)
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
						fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted BSD style checksum line\n", file, lineNum)
					}
					hasError = true
					continue
				}
				// Extract filename from "MD5 (filename)"
				middle := strings.TrimPrefix(parts[0], "MD5 (")
				if !strings.HasSuffix(middle, ")") {
					if warn {
						fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted BSD style checksum line\n", file, lineNum)
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
						fmt.Fprintf(os.Stderr, "md5sum: %s:%d: improperly formatted checksum line\n", file, lineNum)
					}
					hasError = true
					continue
				}
				expectedHash = parts[0]
				// Filename might have spaces, so join remaining parts
				filename = strings.Join(parts[1:], " ")
			}

			// Compute actual hash
			fileToCheck, err := os.Open(filename)
			if err != nil {
				if !quiet {
					fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", filename, err)
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

			if !quiet {
				if actualHash == expectedHash {
					fmt.Printf("%s: OK\n", filename)
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
				fmt.Fprintf(os.Stderr, "md5sum: %s: error reading: %v\n", file, err)
			}
			hasError = true
		}

	}

	if status && hasError {
		return errors.New("checksum verification failed")
	}
	return nil
}
