package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// findCmd implements a basic subset of busybox find
func findCmd(args []string) error {
	fsFlags := flag.NewFlagSet("find", flag.ContinueOnError)
	name := fsFlags.String("name", "", "match basename with pattern (shell glob)")
	typ := fsFlags.String("type", "", "file type: f (file) or d (dir)")
	maxdepth := fsFlags.Int("maxdepth", -1, "maximum depth")
	mindepth := fsFlags.Int("mindepth", 0, "minimum depth")
	printFlag := fsFlags.Bool("print", true, "print matched paths")
	empty := fsFlags.Bool("empty", false, "match empty files or directories")
	size := fsFlags.String("size", "", "file size: +N (larger than N), -N (smaller than N), N (equal to N) (K/M/G suffixes supported)")
	atime := fsFlags.String("atime", "", "file access time: +N, -N, N (N[smh] = seconds/minutes/hours/days)")
	mtime := fsFlags.String("mtime", "", "file modify time: +N, -N, N (N[smh] = seconds/minutes/hours/days)")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox find [OPTIONS] [PATH...]")
		fmt.Fprintln(os.Stderr, "Search for files in a directory hierarchy.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fsFlags.PrintDefaults()
	}

	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
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
		fmt.Fprintf(os.Stderr, "DEBUG: paths=%v, name='%s', typ='%s', maxdepth=%d, mindepth=%d, empty=%v, size='%s', atime='%s', mtime='%s'\n",
			paths, *name, *typ, *maxdepth, *mindepth, *empty, *size, *atime, *mtime)
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

			// type filter
			if *typ != "" {
				if *typ == "f" && d.IsDir() {
					return nil
				}
				if *typ == "d" && !d.IsDir() {
					return nil
				}
			}

			// name filter (glob pattern matching, not regex)
			if *name != "" {
				// Use filepath.Match for glob pattern matching
				// Patterns: * matches any sequence, ? matches any single char, [abc] matches char class
				matched, err := filepath.Match(*name, d.Name())
				if err != nil || !matched {
					return nil
				}
			}

			// empty filter
			if *empty {
				if d.IsDir() {
					// Check if directory is empty
					entries, err := os.ReadDir(p)
					if err != nil {
						return nil
					}
					if len(entries) > 0 {
						return nil
					}
				} else {
					// Check if file is empty
					info, err := d.Info()
					if err != nil {
						return nil
					}
					if info.Size() > 0 {
						return nil
					}
				}
			}

			// size filter
			if *size != "" {
				if d.IsDir() {
					// Skip size filtering for directories
				} else {
					info, err := d.Info()
					if err != nil {
						return nil
					}
					fileSize := info.Size()
					if !matchSize(fileSize, *size) {
						return nil
					}
				}
			}

			// atime filter (access time)
			if *atime != "" {
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if !matchTime(info, *atime, "atime") {
					return nil
				}
			}

			// mtime filter (modify time)
			if *mtime != "" {
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if !matchTime(info, *mtime, "mtime") {
					return nil
				}
			}

			if *printFlag {
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
// +N: newer than N (less than N time units ago)
// -N: older than N (more than N time units ago)
// N: exactly N (within N time units)
// Units: s (seconds), m (minutes), h (hours), d (days, default)
func parseTime(timeSpec string) (time.Duration, int, error) {
	if timeSpec == "" {
		return 0, 0, fmt.Errorf("time specification is empty")
	}

	operator := 0 // 0 = exact, 1 = newer (less than), -1 = older (more than)
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
	case 1: // newer than (less than N units ago)
		return timeSinceFileTime < targetDuration
	case -1: // older than (more than N units ago)
		return timeSinceFileTime > targetDuration
	case 0: // exactly N
		// Within a tolerance of the time unit
		return timeSinceFileTime >= 0 && timeSinceFileTime <= targetDuration
	default:
		return false
	}
}
