package fs

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"gobox/cmds/utils"
)

type duExcludePatterns []string

func (p *duExcludePatterns) String() string {
	return strings.Join(*p, ",")
}

func (p *duExcludePatterns) Set(value string) error {
	*p = append(*p, value)
	return nil
}

type duOptions struct {
	human        bool
	summary      bool
	all          bool
	total        bool
	maxDepth     int
	excludes     []string
	oneFS        bool
	apparentSize bool
}

type duRow struct {
	path string
	size int64
}

func DuCmd(args []string) error {
	fsFlags := flag.NewFlagSet("du", flag.ContinueOnError)
	var opts duOptions
	var excludes duExcludePatterns
	fsFlags.BoolVar(&opts.human, "h", false, "human readable sizes")
	fsFlags.BoolVar(&opts.summary, "s", false, "summarize")
	fsFlags.BoolVar(&opts.all, "a", false, "write counts for all files")
	fsFlags.BoolVar(&opts.total, "c", false, "produce a grand total")
	fsFlags.IntVar(&opts.maxDepth, "d", -1, "print directories at most N levels deep")
	fsFlags.IntVar(&opts.maxDepth, "max-depth", -1, "print directories at most N levels deep")
	fsFlags.Var(&excludes, "exclude", "exclude files matching PATTERN")
	fsFlags.BoolVar(&opts.oneFS, "x", false, "skip directories on different filesystems")
	fsFlags.BoolVar(&opts.apparentSize, "apparent-size", false, "print apparent sizes instead of disk usage")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox du [OPTION]... [PATH...]")
		fmt.Fprintln(os.Stderr, "Summarize disk usage of the set of FILEs, recursively for directories.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -h                    human readable sizes")
		fmt.Fprintln(os.Stderr, "  -s                    summarize each argument")
		fmt.Fprintln(os.Stderr, "  -a                    write counts for all files")
		fmt.Fprintln(os.Stderr, "  -c                    produce a grand total")
		fmt.Fprintln(os.Stderr, "  -d, --max-depth N     print directories at most N levels deep")
		fmt.Fprintln(os.Stderr, "  --exclude PATTERN     exclude files matching PATTERN")
		fmt.Fprintln(os.Stderr, "  -x                    skip directories on different filesystems")
		fmt.Fprintln(os.Stderr, "  --apparent-size       print apparent sizes instead of disk usage")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox du -sh .")
		fmt.Fprintln(os.Stderr, "  gobox du --max-depth 2 --exclude '*.tmp' /var")
	}

	if err := fsFlags.Parse(expandDuBundledFlags(args)); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	paths := fsFlags.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	opts.excludes = excludes

	var grandTotal int64
	for _, root := range paths {
		rows, total, err := collectDiskUsage(root, opts)
		if err != nil {
			return err
		}
		grandTotal += total
		if opts.summary {
			printDuRow(total, root, opts.human)
			continue
		}
		for _, row := range rows {
			printDuRow(row.size, row.path, opts.human)
		}
	}
	if opts.total {
		printDuRow(grandTotal, "total", opts.human)
	}
	return nil
}

// duBundledBoolFlags lists du's single-character boolean flags that are
// safe to bundle together, e.g. "-sh" meaning "-s -h".
var duBundledBoolFlags = map[byte]bool{
	'h': true,
	's': true,
	'a': true,
	'c': true,
	'x': true,
}

// expandDuBundledFlags splits bundled short boolean flags such as "-sh"
// into separate "-s -h" tokens before they reach Go's flag package, which
// does not support bundling on its own. Only single-dash tokens made up
// entirely of known boolean short flags are expanded; long flags (--foo)
// and flags carrying a value (like -d) are left untouched.
func expandDuBundledFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
			bundle := true
			for i := 1; i < len(arg); i++ {
				if !duBundledBoolFlags[arg[i]] {
					bundle = false
					break
				}
			}
			if bundle {
				for i := 1; i < len(arg); i++ {
					out = append(out, "-"+string(arg[i]))
				}
				continue
			}
		}
		out = append(out, arg)
	}
	return out
}

func diskUsage(root string) (int64, error) {
	_, total, err := collectDiskUsage(root, duOptions{})
	return total, err
}

func collectDiskUsage(root string, opts duOptions) ([]duRow, int64, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return nil, 0, err
	}
	var rootDev uint64
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		rootDev = uint64(st.Dev)
	}
	rows := []duRow{}
	total, err := walkDu(root, info, 0, root, rootDev, opts, &rows)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func walkDu(path string, info fs.FileInfo, depth int, root string, rootDev uint64, opts duOptions, rows *[]duRow) (int64, error) {
	excluded, err := excludedDuPath(root, path, opts.excludes)
	if err != nil {
		return 0, err
	}
	if excluded {
		return 0, nil
	}
	if opts.oneFS && depth > 0 {
		if st, ok := info.Sys().(*syscall.Stat_t); ok && uint64(st.Dev) != rootDev {
			return 0, nil
		}
	}

	total := duFileSize(info, opts.apparentSize)
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err == nil {
			sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
			for _, entry := range entries {
				child := filepath.Join(path, entry.Name())
				childInfo, err := os.Lstat(child)
				if err != nil {
					continue
				}
				childTotal, err := walkDu(child, childInfo, depth+1, root, rootDev, opts, rows)
				if err != nil {
					return 0, err
				}
				total += childTotal
			}
		}
		if opts.maxDepth < 0 || depth <= opts.maxDepth {
			*rows = append(*rows, duRow{path: path, size: total})
		}
		return total, nil
	}

	if (opts.all || depth == 0) && (opts.maxDepth < 0 || depth <= opts.maxDepth) {
		*rows = append(*rows, duRow{path: path, size: total})
	}
	return total, nil
}

// excludedDuPath reports whether path should be skipped given --exclude
// patterns. Matching follows GNU du/fnmatch precedence: a pattern containing
// "/" matches only against the path relative to root; a pattern without "/"
// matches only against the entry's basename (at any depth). The root
// argument is cleaned first so behavior doesn't depend on whether it was
// spelled as an absolute or relative path.
func excludedDuPath(root, path string, patterns []string) (bool, error) {
	if len(patterns) == 0 {
		return false, nil
	}
	rel, err := filepath.Rel(filepath.Clean(root), path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(path)
	for _, pattern := range patterns {
		slashPattern := filepath.ToSlash(pattern)
		target := base
		if strings.Contains(slashPattern, "/") {
			target = rel
		}
		ok, err := filepath.Match(slashPattern, target)
		if err != nil {
			return false, fmt.Errorf("du: invalid pattern %q: %w", pattern, err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func duFileSize(info fs.FileInfo, apparent bool) int64 {
	if apparent {
		return info.Size()
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

func printDuRow(size int64, path string, human bool) {
	if human {
		fmt.Printf("%s\t%s\n", utils.HumanSize(size), path)
		return
	}
	blocks := (size + 1023) / 1024
	fmt.Printf("%d\t%s\n", blocks, path)
}
