package fs

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
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
		fmt.Fprintln(os.Stderr, "  -c, --format FORMAT  use custom format string (see directives below)")
		fmt.Fprintln(os.Stderr, "  -t, --terse          terse output")
		fmt.Fprintln(os.Stderr, "  -h, --help           show this help")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Format directives:")
		fmt.Fprintf(os.Stderr, "%s\n", "  %n  filename              %s  size in bytes")
		fmt.Fprintf(os.Stderr, "%s\n", "  %f  raw mode (hex)        %F  file type")
		fmt.Fprintf(os.Stderr, "%s\n", "  %u  user ID               %g  group ID")
		fmt.Fprintf(os.Stderr, "%s\n", "  %U  user name             %G  group name")
		fmt.Fprintln(os.Stderr, "  %a  access rights (octal) %A  access rights (human-readable)")
		fmt.Fprintln(os.Stderr, "  %i  inode number          %h  number of hard links")
		fmt.Fprintf(os.Stderr, "%s\n", "  %d  device number (dec)   %D  device number (hex)")
		fmt.Fprintf(os.Stderr, "%s\n", "  %o  I/O block size        %b  number of blocks")
		fmt.Fprintf(os.Stderr, "%s\n", "  %X  last access (epoch)   %x  last access (readable)")
		fmt.Fprintln(os.Stderr, "  %Y  last modify (epoch)   %y  last modify (readable)")
		fmt.Fprintln(os.Stderr, "  %Z  last change (epoch)   %z  last change (readable)")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox stat file.txt")
		fmt.Fprintln(os.Stderr, "  gobox stat -f /tmp")
		fmt.Fprintf(os.Stderr, "%s\n", "  gobox stat -c '%n %s %y' file.txt")
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
			printStatTerse(file, info)
		} else {
			printStatDefault(file, info)
		}
	}
	return nil
}

// printStatDefault prints file metadata in GNU coreutils' default multi-line
// stat format: File/Size/Device/Inode/Access/Modify/Change, matching the
// well-known layout users expect from real `stat FILE`.
func printStatDefault(file string, info os.FileInfo) {
	st, _ := info.Sys().(*syscall.Stat_t)
	var dev, ino, nlink uint64
	var uid, gid uint32
	var blocks, blksize int64 = 0, 4096
	var atim, mtim, ctim syscall.Timespec
	if st != nil {
		dev, ino, nlink = st.Dev, st.Ino, st.Nlink
		uid, gid = st.Uid, st.Gid
		blocks, blksize = st.Blocks, st.Blksize
		atim, mtim, ctim = st.Atim, st.Mtim, st.Ctim
	}
	perm := permString(info.Mode())
	fullMode := statFullOctal(info.Mode())

	fmt.Printf("  File: %s\n", file)
	fmt.Printf("  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n", info.Size(), blocks, blksize, fileType(info))
	fmt.Printf("Device: %xh/%dd\tInode: %-11d Links: %d\n", dev, dev, ino, nlink)
	fmt.Printf("Access: (%04o/%s)  Uid: (%5d/%8s)   Gid: (%5d/%8s)\n", fullMode, perm, uid, lookupUserName(uid), gid, lookupGroupName(gid))
	fmt.Printf("Access: %s\n", statTimeString(atim))
	fmt.Printf("Modify: %s\n", statTimeString(mtim))
	fmt.Printf("Change: %s\n", statTimeString(ctim))
}

// printStatTerse prints file metadata the way GNU coreutils' `stat -t` does:
// a single space-separated line of raw field values in the fixed order
// "name size blocks rawmode(hex) uid gid device(hex) inode links
// major(hex) minor(hex) atime mtime ctime birthtime blksize" (no field
// labels, no formatted dates). Birthtime is not tracked (see
// printStatDefault) so it is always reported as 0, matching an unsupported
// birth time on native stat.
func printStatTerse(file string, info os.FileInfo) {
	st, _ := info.Sys().(*syscall.Stat_t)
	var dev, ino, nlink, rdev uint64
	var uid, gid, rawMode uint32
	var blocks, blksize int64 = 0, 4096
	var atim, mtim, ctim syscall.Timespec
	if st != nil {
		dev, ino, nlink = st.Dev, st.Ino, st.Nlink
		uid, gid = st.Uid, st.Gid
		blocks, blksize = st.Blocks, st.Blksize
		atim, mtim, ctim = st.Atim, st.Mtim, st.Ctim
		rawMode = st.Mode
		rdev = uint64(st.Rdev)
	}
	major, minor := gnuDevMajor(rdev), gnuDevMinor(rdev)
	fmt.Printf("%s %d %d %x %d %d %x %d %d %x %x %d %d %d %d %d\n",
		file, info.Size(), blocks, rawMode, uid, gid, dev, ino, nlink,
		major, minor, atim.Sec, mtim.Sec, ctim.Sec, 0, blksize)
}

// gnuDevMajor and gnuDevMinor extract the major/minor device numbers from a
// raw dev_t the same way glibc's gnu_dev_major/gnu_dev_minor (used by GNU
// coreutils' stat -t) do; for regular files rdev is 0 so both are 0.
func gnuDevMajor(dev uint64) uint64 {
	return ((dev >> 8) & 0xfff) | ((dev >> 32) &^ 0xfff)
}

func gnuDevMinor(dev uint64) uint64 {
	return (dev & 0xff) | ((dev >> 12) &^ 0xff)
}

// permString builds the 10-character permission string GNU stat/ls show,
// e.g. "-rw-r--r--", including setuid/setgid/sticky bits (s/S, t/T).
func permString(mode os.FileMode) string {
	var typeChar byte
	switch {
	case mode&os.ModeSymlink != 0:
		typeChar = 'l'
	case mode.IsDir():
		typeChar = 'd'
	case mode&os.ModeNamedPipe != 0:
		typeChar = 'p'
	case mode&os.ModeSocket != 0:
		typeChar = 's'
	case mode&os.ModeCharDevice != 0:
		typeChar = 'c'
	case mode&os.ModeDevice != 0:
		typeChar = 'b'
	default:
		typeChar = '-'
	}
	b := []byte{typeChar, '-', '-', '-', '-', '-', '-', '-', '-', '-'}
	const rwx = "rwxrwxrwx"
	perm := mode.Perm()
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			b[1+i] = rwx[i]
		}
	}
	if mode&os.ModeSetuid != 0 {
		if b[3] == 'x' {
			b[3] = 's'
		} else {
			b[3] = 'S'
		}
	}
	if mode&os.ModeSetgid != 0 {
		if b[6] == 'x' {
			b[6] = 's'
		} else {
			b[6] = 'S'
		}
	}
	if mode&os.ModeSticky != 0 {
		if b[9] == 'x' {
			b[9] = 't'
		} else {
			b[9] = 'T'
		}
	}
	return string(b)
}

// statFullOctal returns the permission bits plus setuid/setgid/sticky as a
// single octal number the way GNU stat's %a/default output shows it (e.g.
// 4755 for a setuid binary), not just the low 9 permission bits.
func statFullOctal(mode os.FileMode) uint32 {
	m := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		m |= 04000
	}
	if mode&os.ModeSetgid != 0 {
		m |= 02000
	}
	if mode&os.ModeSticky != 0 {
		m |= 01000
	}
	return m
}

func statTimeString(ts syscall.Timespec) string {
	return time.Unix(ts.Sec, ts.Nsec).Format("2006-01-02 15:04:05.000000000 -0700")
}

func lookupUserName(uid uint32) string {
	if u, err := user.LookupId(strconv.FormatUint(uint64(uid), 10)); err == nil {
		return u.Username
	}
	return strconv.FormatUint(uint64(uid), 10)
}

func lookupGroupName(gid uint32) string {
	if g, err := user.LookupGroupId(strconv.FormatUint(uint64(gid), 10)); err == nil {
		return g.Name
	}
	return strconv.FormatUint(uint64(gid), 10)
}

// formatStat renders a stat -c/--format FORMAT string against file. It
// supports the common GNU stat file-mode directives and %% for a literal
// percent sign; unrecognized directives are left as-is (directive char
// included) rather than silently dropped.
func formatStat(format, name string, info os.FileInfo) string {
	st, _ := info.Sys().(*syscall.Stat_t)
	var dev, ino, nlink uint64
	var uid, gid, rawMode uint32
	var blocks, blksize int64
	var atim, mtim, ctim syscall.Timespec
	if st != nil {
		dev, ino, nlink = st.Dev, st.Ino, st.Nlink
		uid, gid = st.Uid, st.Gid
		blocks, blksize = st.Blocks, st.Blksize
		atim, mtim, ctim = st.Atim, st.Mtim, st.Ctim
		rawMode = st.Mode
	}

	var out strings.Builder
	runes := []rune(format)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '%' || i == len(runes)-1 {
			out.WriteRune(runes[i])
			continue
		}
		i++
		switch runes[i] {
		case '%':
			out.WriteByte('%')
		case 'n':
			out.WriteString(name)
		case 's':
			fmt.Fprintf(&out, "%d", info.Size())
		case 'f':
			fmt.Fprintf(&out, "%x", rawMode)
		case 'F':
			out.WriteString(fileType(info))
		case 'u':
			fmt.Fprintf(&out, "%d", uid)
		case 'g':
			fmt.Fprintf(&out, "%d", gid)
		case 'U':
			out.WriteString(lookupUserName(uid))
		case 'G':
			out.WriteString(lookupGroupName(gid))
		case 'a':
			fmt.Fprintf(&out, "%o", info.Mode().Perm())
		case 'A':
			out.WriteString(permString(info.Mode()))
		case 'X':
			fmt.Fprintf(&out, "%d", atim.Sec)
		case 'Y':
			fmt.Fprintf(&out, "%d", mtim.Sec)
		case 'Z':
			fmt.Fprintf(&out, "%d", ctim.Sec)
		case 'x':
			out.WriteString(statTimeString(atim))
		case 'y':
			out.WriteString(info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700"))
		case 'z':
			out.WriteString(statTimeString(ctim))
		case 'i':
			fmt.Fprintf(&out, "%d", ino)
		case 'h':
			fmt.Fprintf(&out, "%d", nlink)
		case 'd':
			fmt.Fprintf(&out, "%d", dev)
		case 'D':
			fmt.Fprintf(&out, "%x", dev)
		case 'o':
			fmt.Fprintf(&out, "%d", blksize)
		case 'b':
			fmt.Fprintf(&out, "%d", blocks)
		default:
			out.WriteByte('%')
			out.WriteRune(runes[i])
		}
	}
	return out.String()
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
		fmt.Printf("  File: %q\n", path)
		fmt.Printf("    ID: %-8s Namelen: %-7d Type: %s\n", formatFsid(st.Fsid), st.Namelen, statFSTypeName(st.Type))
		fmt.Printf("Block size: %-10d Fundamental block size: %d\n", st.Bsize, st.Bsize)
		fmt.Printf("Blocks: Total: %-10d Free: %-10d Available: %d\n", st.Blocks, st.Bfree, st.Bavail)
		fmt.Printf("Inodes: Total: %-10d Free: %d\n", st.Files, st.Ffree)
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
