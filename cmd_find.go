package main

import (
    "flag"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
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
                if d.IsDir() {
                    return nil
                }
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

            // name filter
            if *name != "" {
                matched, _ := filepath.Match(*name, d.Name())
                if !matched {
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
