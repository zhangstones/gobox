package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func topCmd(args []string) error {
	fsFlags := flag.NewFlagSet("top", flag.ContinueOnError)
	interval := fsFlags.Int("d", 2, "delay in seconds between updates")
	count := fsFlags.Int("n", 5, "number of iterations (0 = infinite)")
	// sorting options (keep consistent with cmd_ps.go)
	sortBy := fsFlags.String("sort", "pid", "sort by: pid|cpu|rss|vms|cmd")
	rev := fsFlags.Bool("r", true, "reverse sort order")

	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox top [OPTIONS]")
		fmt.Fprintln(os.Stderr, "Display dynamic real-time view of running processes (very small subset).")
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

	iterations := *count
	if iterations < 0 {
		iterations = 0
	}

	i := 0
	for {
		// clear screen (best-effort)
		fmt.Print("\033[H\033[2J")
		// forward selected sorting flags to psCmd so behavior matches cmd_ps.go
		psArgs := []string{"-f", "-sort", *sortBy}
		if *rev {
			psArgs = append(psArgs, "-r")
		}
		_ = psCmd(psArgs)
		i++
		if iterations != 0 && i >= iterations {
			break
		}
		time.Sleep(time.Duration(*interval) * time.Second)
	}
	return nil
}
