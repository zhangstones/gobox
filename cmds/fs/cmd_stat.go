package fs

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var statFSTypeNames = map[int64]string{
	0x01021994: "tmpfs",
	0x00009fa0: "proc",
	0x00001cd1: "devtmpfs",
	0x0000ef53: "ext2/ext3",
	0x0027e0eb: "cgroup",
	0x63677270: "cgroup2fs",
	0x62656572: "sysfs",
	0x73717368: "squashfs",
	0x794c7630: "overlay",
	0x9123683e: "btrfs",
	0x58465342: "xfs",
}

func StatCmd(args []string) error {
	fsFlags := flag.NewFlagSet("stat", flag.ContinueOnError)
	deref := fsFlags.Bool("L", false, "follow links")
	fsFlags.BoolVar(deref, "dereference", false, "follow links")
	fileSystem := fsFlags.Bool("f", false, "display filesystem status")
	fsFlags.BoolVar(fileSystem, "file-system", false, "display filesystem status")
	format := fsFlags.String("c", "", "use FORMAT")
	fsFlags.StringVar(format, "format", "", "use FORMAT")
	terse := fsFlags.Bool("t", false, "terse output")
	fsFlags.BoolVar(terse, "terse", false, "terse output")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox stat [OPTION]... FILE...")
		fmt.Fprintln(os.Stderr, "Display file or filesystem status.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -L, --dereference    follow links")
		fmt.Fprintln(os.Stderr, "  -f, --file-system    display filesystem status")
		fmt.Fprintln(os.Stderr, "  -c, --format FORMAT  use custom format string")
		fmt.Fprintln(os.Stderr, "  -t, --terse          terse output")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox stat file.txt")
		fmt.Fprintln(os.Stderr, "  gobox stat -f /tmp")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	files := fsFlags.Args()
	if len(files) == 0 {
		return fmt.Errorf("missing operand")
	}
	for _, file := range files {
		if *fileSystem {
			if err := printStatFS(file, *format, *terse); err != nil {
				return err
			}
			continue
		}
		var info os.FileInfo
		var err error
		if *deref {
			info, err = os.Stat(file)
		} else {
			info, err = os.Lstat(file)
		}
		if err != nil {
			return err
		}
		if *format != "" {
			fmt.Println(formatStat(*format, file, info))
		} else if *terse {
			fmt.Printf("%s %d %o %s\n", file, info.Size(), info.Mode().Perm(), info.ModTime().Format(time.RFC3339))
		} else {
			fmt.Printf("  File: %s\n", file)
			fmt.Printf("  Size: %d\tMode: %04o\tType: %s\n", info.Size(), info.Mode().Perm(), fileType(info))
			fmt.Printf("Modify: %s\n", info.ModTime().Format(time.RFC3339))
		}
	}
	return nil
}

func formatStat(format, name string, info os.FileInfo) string {
	out := format
	out = strings.ReplaceAll(out, "%n", name)
	out = strings.ReplaceAll(out, "%s", fmt.Sprintf("%d", info.Size()))
	out = strings.ReplaceAll(out, "%F", fileType(info))
	out = strings.ReplaceAll(out, "%a", fmt.Sprintf("%o", info.Mode().Perm()))
	out = strings.ReplaceAll(out, "%y", info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700"))
	out = strings.ReplaceAll(out, "%Y", fmt.Sprintf("%d", info.ModTime().Unix()))
	return out
}

func fileType(info os.FileInfo) string {
	mode := info.Mode()
	switch {
	case mode.IsRegular():
		return "regular file"
	case mode.IsDir():
		return "directory"
	case mode&os.ModeSymlink != 0:
		return "symbolic link"
	default:
		return mode.Type().String()
	}
}

func printStatFS(path, format string, terse bool) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("stat -f supported only on Linux")
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return err
	}
	if format != "" {
		out := strings.ReplaceAll(format, "%n", path)
		out = strings.ReplaceAll(out, "%s", fmt.Sprintf("%d", st.Bsize))
		out = strings.ReplaceAll(out, "%b", fmt.Sprintf("%d", st.Blocks))
		out = strings.ReplaceAll(out, "%f", fmt.Sprintf("%d", st.Bfree))
		fmt.Println(out)
		return nil
	}
	if terse {
		fmt.Printf("%s %d %d %d\n", path, st.Bsize, st.Blocks, st.Bfree)
	} else {
		fmt.Printf("  File: %s\n", path)
		fmt.Printf("    ID: %s Namelen: %d Type: %s\n", formatFsid(st.Fsid), st.Namelen, statFSTypeName(st.Type))
		fmt.Printf("Block size: %d Blocks: %d Free: %d Available: %d\n", st.Bsize, st.Blocks, st.Bfree, st.Bavail)
	}
	return nil
}

// formatFsid renders a filesystem ID the way GNU coreutils' stat does:
// the two 32-bit words of f_fsid concatenated as hex (first word unpadded,
// second word zero-padded to 8 digits), e.g. "fd0000000000".
func formatFsid(fsid syscall.Fsid) string {
	return fmt.Sprintf("%x%08x", uint32(fsid.X__val[0]), uint32(fsid.X__val[1]))
}

func statFSTypeName(fsType int64) string {
	if name, ok := statFSTypeNames[fsType]; ok {
		return name
	}
	return fmt.Sprintf("%x", fsType)
}
