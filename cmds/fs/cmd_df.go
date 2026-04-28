package fs

import (
	"bufio"
	"flag"
	"fmt"
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
		if !opts.all && !opts.inodes && row.stat.Blocks == 0 {
			continue
		}
		rows = append(rows, row)
	}
	if opts.total {
		rows = append(rows, totalDfRow(rows, opts))
	}
	printDfHeader(rows, opts)
	for _, row := range rows {
		printDfRow(row, rows, opts)
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

func printDfHeader(rows []dfRow, opts dfOptions) {
	sourceWidth, typeWidth := dfColumnWidths(rows, opts)
	blockHeader := "1K-blocks"
	if opts.human {
		blockHeader = "Size"
	}
	if opts.si {
		blockHeader = "Size"
	}
	if opts.posix && !opts.human && !opts.si {
		blockHeader = "1024-blocks"
	}
	if opts.inodes {
		if opts.showType {
			fmt.Printf("%-*s %-*s %10s %10s %10s %5s %s\n", sourceWidth, "Filesystem", typeWidth, "Type", "Inodes", "IUsed", "IFree", "IUse%", "Mounted on")
			return
		}
		fmt.Printf("%-*s %10s %10s %10s %5s %s\n", sourceWidth, "Filesystem", "Inodes", "IUsed", "IFree", "IUse%", "Mounted on")
		return
	}
	if opts.showType {
		fmt.Printf("%-*s %-*s %10s %10s %10s %5s %s\n", sourceWidth, "Filesystem", typeWidth, "Type", blockHeader, "Used", "Available", "Use%", "Mounted on")
		return
	}
	fmt.Printf("%-*s %10s %10s %10s %5s %s\n", sourceWidth, "Filesystem", blockHeader, "Used", "Available", "Use%", "Mounted on")
}

func printDfRow(row dfRow, rows []dfRow, opts dfOptions) {
	sourceWidth, typeWidth := dfColumnWidths(rows, opts)
	m := row.mount
	st := row.stat
	if opts.inodes {
		used := st.Files - st.Ffree
		if opts.showType {
			fmt.Printf("%-*s %-*s %10d %10d %10d %5s %s\n", sourceWidth, m.Source, typeWidth, m.FSType, st.Files, used, st.Ffree, percent(used, st.Files), m.Target)
			return
		}
		fmt.Printf("%-*s %10d %10d %10d %5s %s\n", sourceWidth, m.Source, st.Files, used, st.Ffree, percent(used, st.Files), m.Target)
		return
	}
	blockSize := uint64(st.Bsize)
	total := st.Blocks * blockSize
	free := st.Bavail * blockSize
	used := total - free
	totalText, usedText, freeText := formatDfSize(total, used, free, opts)
	if opts.showType {
		fmt.Printf("%-*s %-*s %10s %10s %10s %5s %s\n", sourceWidth, m.Source, typeWidth, m.FSType, totalText, usedText, freeText, percent(used, total), m.Target)
		return
	}
	fmt.Printf("%-*s %10s %10s %10s %5s %s\n", sourceWidth, m.Source, totalText, usedText, freeText, percent(used, total), m.Target)
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

func humanSizeBase(b uint64, unit uint64) string {
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	value := float64(b) / float64(div)
	suf := "KMGTPE"[exp]
	if unit == 1000 {
		return fmt.Sprintf("%.1f%c", value, suf)
	}
	return fmt.Sprintf("%.1f%c", value, suf)
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
	}
	return total
}

func percent(used, total uint64) string {
	if total == 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", (used*100+total-1)/total)
}
