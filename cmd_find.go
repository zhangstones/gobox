package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	paths := fsFlags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Debug output
	if os.Getenv("DEBUG_FIND") != "" {
		fmt.Fprintf(os.Stderr, "DEBUG: paths=%v, name='%s', typ='%s', maxdepth=%d, mindepth=%d, empty=%v, size='%s'\n",
			paths, *name, *typ, *maxdepth, *mindepth, *empty, *size)
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
