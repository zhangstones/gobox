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
		fsFlags.PrintDefaults()
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
	repl := map[string]string{
		"%n": name,
		"%s": fmt.Sprintf("%d", info.Size()),
		"%F": fileType(info),
		"%a": fmt.Sprintf("%o", info.Mode().Perm()),
		"%y": info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700"),
		"%Y": fmt.Sprintf("%d", info.ModTime().Unix()),
	}
	out := format
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
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
		fmt.Printf("    ID: %x Namelen: %d Type: %x\n", st.Fsid, st.Namelen, st.Type)
		fmt.Printf("Block size: %d Blocks: %d Free: %d Available: %d\n", st.Bsize, st.Blocks, st.Bfree, st.Bavail)
	}
	return nil
}
