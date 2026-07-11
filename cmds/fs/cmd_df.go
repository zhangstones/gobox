package fs

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

type mountInfo struct {
	Source string
	Target string
	FSType string
}

var (
	dfGOOS       = runtime.GOOS
	readMounts   = readMountInfo
	statDfPath   = os.Stat
	statfsDfPath = syscall.Statfs
)

type dfTypeFilter []string

func (f *dfTypeFilter) String() string {
	return strings.Join(*f, ",")
}

func (f *dfTypeFilter) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type dfOptions struct {
	human       bool
	si          bool
	showType    bool
	inodes      bool
	all         bool
	local       bool
	includeType []string
	excludeType []string
	total       bool
	posix       bool
}

type dfRow struct {
	mount mountInfo
	stat  syscall.Statfs_t
}

func DfCmd(args []string) error {
	fsFlags := flag.NewFlagSet("df", flag.ContinueOnError)
	var opts dfOptions
	var includeTypes dfTypeFilter
	var excludeTypes dfTypeFilter
	fsFlags.BoolVar(&opts.human, "h", false, "human readable")
	fsFlags.BoolVar(&opts.si, "H", false, "human readable SI units")
	fsFlags.BoolVar(&opts.showType, "T", false, "show filesystem type")
	fsFlags.BoolVar(&opts.inodes, "i", false, "show inode usage")
	fsFlags.BoolVar(&opts.all, "a", false, "include all filesystems")
	fsFlags.BoolVar(&opts.local, "l", false, "limit listing to local filesystems")
	fsFlags.Var(&includeTypes, "t", "limit listing to filesystems of type TYPE")
	fsFlags.Var(&excludeTypes, "x", "exclude filesystems of type TYPE")
	fsFlags.BoolVar(&opts.total, "total", false, "produce a grand total")
	fsFlags.BoolVar(&opts.posix, "P", false, "use POSIX output format")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox df [OPTION]... [PATH...]")
		fmt.Fprintln(os.Stderr, "Report filesystem disk space usage.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Display:")
		fmt.Fprintln(os.Stderr, "  -h               human readable units")
		fmt.Fprintln(os.Stderr, "  -H               human readable SI units")
		fmt.Fprintln(os.Stderr, "  -T               show filesystem type")
		fmt.Fprintln(os.Stderr, "  -i               show inode usage")
		fmt.Fprintln(os.Stderr, "  -P               use POSIX output format")
		fmt.Fprintln(os.Stderr, "  --total          produce a grand total")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Filters:")
		fmt.Fprintln(os.Stderr, "  -a               include all filesystems")
		fmt.Fprintln(os.Stderr, "  -l               limit listing to local filesystems")
		fmt.Fprintln(os.Stderr, "  -t TYPE          limit listing to filesystems of type TYPE")
		fmt.Fprintln(os.Stderr, "  -x TYPE          exclude filesystems of type TYPE")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "      --help       show this help")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if dfGOOS != "linux" {
		return fmt.Errorf("df supported only on Linux")
	}
	opts.includeType = includeTypes
	opts.excludeType = excludeTypes
	mounts, err := readMounts()
	if err != nil {
		return err
	}
	paths := fsFlags.Args()
	if len(paths) == 0 {
		paths = make([]string, 0, len(mounts))
		for _, m := range mounts {
			paths = append(paths, m.Target)
		}
	}

	seen := map[string]bool{}
	rows := []dfRow{}
	var rowErr error
	for _, p := range paths {
		if len(fsFlags.Args()) > 0 {
			if _, err := statDfPath(p); err != nil {
				return fmt.Errorf("df: %s: %w", p, err)
			}
		}
		m := bestMountForPath(mounts, p)
		if !dfMountAllowed(m, opts) {
			continue
		}
		if len(fsFlags.Args()) == 0 && !opts.all && seen[m.Target] {
			continue
		}
		seen[m.Target] = true
		row, err := readDfRow(m)
		if err != nil {
			if len(fsFlags.Args()) == 0 {
				rowErr = err
				continue
			}
			return err
		}
		if !opts.all && row.stat.Blocks == 0 {
			continue
		}
		rows = append(rows, row)
	}
	if opts.total {
		rows = append(rows, totalDfRow(rows, opts))
	}
	sourceWidth, typeWidth := dfColumnWidths(rows, opts)
	col1Header, col2Header, col3Header, pctHeader := dfColumnHeaders(opts)
	w1, w2, w3, w4 := dfNumericWidths(rows, opts, col1Header, col2Header, col3Header, pctHeader)
	printDfHeader(sourceWidth, typeWidth, w1, w2, w3, w4, opts)
	for _, row := range rows {
		printDfRow(row, sourceWidth, typeWidth, w1, w2, w3, w4, opts)
	}
	if len(rows) == 0 && rowErr != nil {
		return rowErr
	}
	return nil
}

func readMountInfo() ([]mountInfo, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var mounts []mountInfo
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " - ")
		if len(parts) != 2 {
			continue
		}
		left := strings.Fields(parts[0])
		right := strings.Fields(parts[1])
		if len(left) < 5 || len(right) < 3 {
			continue
		}
		mounts = append(mounts, mountInfo{Target: decodeMountField(left[4]), FSType: right[0], Source: right[1]})
	}
	sort.Slice(mounts, func(i, j int) bool { return len(mounts[i].Target) > len(mounts[j].Target) })
	return mounts, scanner.Err()
}

func decodeMountField(s string) string {
	s = strings.ReplaceAll(s, `\040`, " ")
	s = strings.ReplaceAll(s, `\011`, "\t")
	s = strings.ReplaceAll(s, `\012`, "\n")
	s = strings.ReplaceAll(s, `\134`, `\`)
	return s
}

func bestMountForPath(mounts []mountInfo, p string) mountInfo {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	for _, m := range mounts {
		if p == m.Target || strings.HasPrefix(p, strings.TrimRight(m.Target, "/")+"/") {
			return m
		}
	}
	if len(mounts) > 0 {
		return mounts[len(mounts)-1]
	}
	return mountInfo{Target: p, Source: p}
}

func readDfRow(m mountInfo) (dfRow, error) {
	var st syscall.Statfs_t
	if err := statfsDfPath(m.Target, &st); err != nil {
		return dfRow{}, err
	}
	return dfRow{mount: m, stat: st}, nil
}

// dfRowValues returns the pre-formatted strings for a row's three numeric
// columns (blocks/used/available, or inodes/iused/ifree) plus the use%
// column, used both to size columns dynamically and to print rows.
func dfRowValues(row dfRow, opts dfOptions) (col1, col2, col3, pct string) {
	st := row.stat
	if opts.inodes {
		// Some pseudo-filesystems (observed with vboxsf) report Ffree >
		// Files, which structurally shouldn't happen; compute used as a
		// signed value so it doesn't wrap around to a huge unsigned
		// number, and show "-" for the percentage like native df does
		// when the inode accounting is nonsensical.
		used := int64(st.Files) - int64(st.Ffree)
		pct := "-"
		if st.Files > 0 && used >= 0 {
			pct = percent(uint64(used), st.Files)
		}
		return fmt.Sprintf("%d", st.Files), fmt.Sprintf("%d", used), fmt.Sprintf("%d", st.Ffree), pct
	}
	blockSize := uint64(st.Bsize)
	total := st.Blocks * blockSize
	free := st.Bavail * blockSize
	used := (st.Blocks - st.Bfree) * blockSize
	totalText, usedText, freeText := formatDfSize(total, used, free, opts)
	return totalText, usedText, freeText, percent(used, total)
}

// dfNumericWidths computes native df's dynamic column widths: each column is
// as wide as its header label, or its widest formatted value across all
// rows, whichever is larger (native df never truncates values, and never
// pads wider than necessary either).
func dfNumericWidths(rows []dfRow, opts dfOptions, col1Header, col2Header, col3Header, pctHeader string) (int, int, int, int) {
	w1, w2, w3, w4 := len(col1Header), len(col2Header), len(col3Header), len(pctHeader)
	for _, row := range rows {
		c1, c2, c3, pct := dfRowValues(row, opts)
		if l := len(c1); l > w1 {
			w1 = l
		}
		if l := len(c2); l > w2 {
			w2 = l
		}
		if l := len(c3); l > w3 {
			w3 = l
		}
		if l := len(pct); l > w4 {
			w4 = l
		}
	}
	return w1, w2, w3, w4
}

func dfColumnWidths(rows []dfRow, opts dfOptions) (int, int) {
	sourceWidth := len("Filesystem")
	typeWidth := len("Type")
	for _, row := range rows {
		if l := len(row.mount.Source); l > sourceWidth {
			sourceWidth = l
		}
		if l := len(row.mount.FSType); l > typeWidth {
			typeWidth = l
		}
	}
	return sourceWidth, typeWidth
}

// dfColumnHeaders returns the three numeric column labels for the current
// mode (blocks vs inodes, and the block-size unit label).
func dfColumnHeaders(opts dfOptions) (col1, col2, col3, pctHeader string) {
	if opts.inodes {
		return "Inodes", "IUsed", "IFree", "IUse%"
	}
	blockHeader := "1K-blocks"
	availHeader := "Available"
	pctHeader = "Use%"
	if opts.human || opts.si {
		blockHeader = "Size"
		// Native df abbreviates "Available" to "Avail" once the size
		// column itself is abbreviated (human/SI units).
		availHeader = "Avail"
	}
	if opts.posix && !opts.human && !opts.si {
		blockHeader = "1024-blocks"
		// Strict POSIX mode (-P without -h/-H) labels the percentage
		// column "Capacity" instead of "Use%".
		pctHeader = "Capacity"
	}
	return blockHeader, "Used", availHeader, pctHeader
}

func printDfHeader(sourceWidth, typeWidth, w1, w2, w3, w4 int, opts dfOptions) {
	col1, col2, col3, pctHeader := dfColumnHeaders(opts)
	if opts.showType {
		fmt.Printf("%-*s %-*s %*s %*s %*s %*s %s\n", sourceWidth, "Filesystem", typeWidth, "Type", w1, col1, w2, col2, w3, col3, w4, pctHeader, "Mounted on")
		return
	}
	fmt.Printf("%-*s %*s %*s %*s %*s %s\n", sourceWidth, "Filesystem", w1, col1, w2, col2, w3, col3, w4, pctHeader, "Mounted on")
}

func printDfRow(row dfRow, sourceWidth, typeWidth, w1, w2, w3, w4 int, opts dfOptions) {
	m := row.mount
	c1, c2, c3, pct := dfRowValues(row, opts)
	if opts.showType {
		fmt.Printf("%-*s %-*s %*s %*s %*s %*s %s\n", sourceWidth, m.Source, typeWidth, m.FSType, w1, c1, w2, c2, w3, c3, w4, pct, m.Target)
		return
	}
	fmt.Printf("%-*s %*s %*s %*s %*s %s\n", sourceWidth, m.Source, w1, c1, w2, c2, w3, c3, w4, pct, m.Target)
}

func formatDfSize(total, used, free uint64, opts dfOptions) (string, string, string) {
	if opts.si {
		return humanSizeBase(total, 1000), humanSizeBase(used, 1000), humanSizeBase(free, 1000)
	}
	if opts.human {
		return humanSizeBase(total, 1024), humanSizeBase(used, 1024), humanSizeBase(free, 1024)
	}
	return fmt.Sprintf("%d", total/1024), fmt.Sprintf("%d", used/1024), fmt.Sprintf("%d", free/1024)
}

// humanSizeBase formats a byte count the way GNU df -h/-H does: no unit
// suffix (and no decimal) for exactly zero, a bare number for the sub-unit
// byte range, and otherwise an adaptive-precision value with a K/M/G/T/P/E
// suffix (no trailing "B") — one decimal place while the rounded magnitude
// is below 10, none once it reaches 10 or more.
func humanSizeBase(b uint64, unit uint64) string {
	if b == 0 {
		return "0"
	}
	if b < unit {
		return fmt.Sprintf("%d", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	scaled := float64(b) / float64(div)
	rounded := math.Round(scaled*10) / 10
	suf := "KMGTPE"[exp]
	if unit == 1000 && exp == 0 {
		// GNU df's SI (-H) mode uses the strict-SI lowercase "k" for the
		// kilo prefix specifically; mega/giga/etc. stay uppercase.
		suf = 'k'
	}
	if rounded >= 10 {
		return fmt.Sprintf("%.0f%c", rounded, suf)
	}
	return fmt.Sprintf("%.1f%c", rounded, suf)
}

func dfMountAllowed(m mountInfo, opts dfOptions) bool {
	if opts.local && !isLocalDfType(m) {
		return false
	}
	if len(opts.includeType) > 0 && !containsDfType(opts.includeType, m.FSType) {
		return false
	}
	if containsDfType(opts.excludeType, m.FSType) {
		return false
	}
	return true
}

func containsDfType(types []string, fsType string) bool {
	for _, typ := range types {
		if typ == fsType {
			return true
		}
	}
	return false
}

func isLocalDfType(m mountInfo) bool {
	if strings.Contains(m.Source, ":") {
		return false
	}
	switch m.FSType {
	case "nfs", "nfs4", "cifs", "smbfs", "sshfs", "fuse.sshfs", "9p", "afs", "ceph", "glusterfs":
		return false
	default:
		return true
	}
}

func totalDfRow(rows []dfRow, opts dfOptions) dfRow {
	total := dfRow{mount: mountInfo{Source: "total", Target: "total", FSType: "-"}}
	for _, row := range rows {
		st := row.stat
		total.stat.Bsize = 1024
		if opts.inodes {
			total.stat.Files += st.Files
			total.stat.Ffree += st.Ffree
			continue
		}
		blockSize := uint64(st.Bsize)
		total.stat.Blocks += (st.Blocks * blockSize) / 1024
		total.stat.Bavail += (st.Bavail * blockSize) / 1024
		total.stat.Bfree += (st.Bfree * blockSize) / 1024
	}
	return total
}

func percent(used, total uint64) string {
	if total == 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", (used*100+total-1)/total)
}
