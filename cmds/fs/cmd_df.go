package fs

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"syscall"

	"gobox/cmds/utils"
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

func DfCmd(args []string) error {
	fsFlags := flag.NewFlagSet("df", flag.ContinueOnError)
	human := fsFlags.Bool("h", false, "human readable")
	showType := fsFlags.Bool("T", false, "show filesystem type")
	inodes := fsFlags.Bool("i", false, "show inode usage")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox df [OPTION]... [PATH...]")
		fsFlags.PrintDefaults()
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
	if *inodes {
		fmt.Printf("%-20s %10s %10s %10s %5s %s\n", "Filesystem", "Inodes", "IUsed", "IFree", "IUse%", "Mounted on")
	} else if *showType {
		fmt.Printf("%-20s %-8s %10s %10s %10s %5s %s\n", "Filesystem", "Type", "1K-blocks", "Used", "Available", "Use%", "Mounted on")
	} else {
		fmt.Printf("%-20s %10s %10s %10s %5s %s\n", "Filesystem", "1K-blocks", "Used", "Available", "Use%", "Mounted on")
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if len(fsFlags.Args()) > 0 {
			if _, err := statDfPath(p); err != nil {
				return fmt.Errorf("df: %s: %w", p, err)
			}
		}
		m := bestMountForPath(mounts, p)
		if len(fsFlags.Args()) == 0 && seen[m.Target] {
			continue
		}
		seen[m.Target] = true
		if err := printDfRow(m, *human, *showType, *inodes); err != nil {
			return err
		}
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
	if abs, err := os.Getwd(); err == nil && !strings.HasPrefix(p, "/") {
		p = abs + "/" + p
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

func printDfRow(m mountInfo, human, showType, inodes bool) error {
	var st syscall.Statfs_t
	if err := statfsDfPath(m.Target, &st); err != nil {
		return err
	}
	if inodes {
		used := st.Files - st.Ffree
		fmt.Printf("%-20s %10d %10d %10d %5s %s\n", m.Source, st.Files, used, st.Ffree, percent(used, st.Files), m.Target)
		return nil
	}
	blockSize := uint64(st.Bsize)
	total := st.Blocks * blockSize
	free := st.Bavail * blockSize
	used := total - free
	if human {
		if showType {
			fmt.Printf("%-20s %-8s %10s %10s %10s %5s %s\n", m.Source, m.FSType, utils.HumanSize(int64(total)), utils.HumanSize(int64(used)), utils.HumanSize(int64(free)), percent(used, total), m.Target)
		} else {
			fmt.Printf("%-20s %10s %10s %10s %5s %s\n", m.Source, utils.HumanSize(int64(total)), utils.HumanSize(int64(used)), utils.HumanSize(int64(free)), percent(used, total), m.Target)
		}
		return nil
	}
	if showType {
		fmt.Printf("%-20s %-8s %10d %10d %10d %5s %s\n", m.Source, m.FSType, total/1024, used/1024, free/1024, percent(used, total), m.Target)
	} else {
		fmt.Printf("%-20s %10d %10d %10d %5s %s\n", m.Source, total/1024, used/1024, free/1024, percent(used, total), m.Target)
	}
	return nil
}

func percent(used, total uint64) string {
	if total == 0 {
		return "-"
	}
	return fmt.Sprintf("%d%%", (used*100+total-1)/total)
}
