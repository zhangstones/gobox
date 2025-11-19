package main

import (
    "flag"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
)

func duCmd(args []string) error {
    fsFlags := flag.NewFlagSet("du", flag.ContinueOnError)
    human := fsFlags.Bool("h", false, "human readable sizes")
    summary := fsFlags.Bool("s", false, "summarize")

    fsFlags.Usage = func() {
        fmt.Fprintln(os.Stderr, "Usage: gobox du [OPTIONS] [PATH...]")
        fmt.Fprintln(os.Stderr, "Summarize disk usage of the set of FILEs, recursively for directories.")
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

    for _, root := range paths {
        total, err := diskUsage(root)
        if err != nil {
            return err
        }
        if *summary {
            if *human {
                fmt.Printf("%s\t%s\n", humanSize(total), root)
            } else {
                fmt.Printf("%d\t%s\n", total, root)
            }
            continue
        }
        // walk and print per-file
        _ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
            if err != nil {
                return nil
            }
            if d.IsDir() {
                return nil
            }
            fi, err := d.Info()
            if err != nil {
                return nil
            }
            size := fi.Size()
            if *human {
                fmt.Printf("%s\t%s\n", humanSize(size), p)
            } else {
                fmt.Printf("%d\t%s\n", size, p)
            }
            return nil
        })
    }
    return nil
}

func diskUsage(root string) (int64, error) {
    var total int64
    err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
        if err != nil {
            return nil
        }
        if d.IsDir() {
            return nil
        }
        fi, err := d.Info()
        if err != nil {
            return nil
        }
        total += fi.Size()
        return nil
    })
    return total, err
}

func humanSize(b int64) string {
    const unit = 1024
    if b < unit {
        return fmt.Sprintf("%dB", b)
    }
    div, exp := int64(unit), 0
    for n := b / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    value := float64(b) / float64(div)
    suf := "KMGTPE"[exp]
    return fmt.Sprintf("%.1f%cB", value, suf)
}
