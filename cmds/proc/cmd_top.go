package proc

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gobox/cmds/utils"
)

func TopCmd(args []string) error {
	fsFlags := flag.NewFlagSet("top", flag.ContinueOnError)
	interval := fsFlags.String("d", "2", "delay in seconds between updates")
	count := fsFlags.Int("n", 5, "number of iterations (0 = infinite)")
	batch := fsFlags.Bool("b", false, "batch mode")
	pids := fsFlags.String("p", "", "show only comma-separated process IDs")
	users := fsFlags.String("u", "", "show only processes for comma-separated users or UIDs")
	threads := fsFlags.Bool("H", false, "thread display mode (accepted; process-level output)")
	hideIdle := fsFlags.Bool("i", false, "hide processes with zero sampled CPU")
	fullCmd := fsFlags.Bool("c", false, "show full command lines")
	orderBy := fsFlags.String("o", "", "sort by field")
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
	_ = threads
	delay, err := parseTopDelay(*interval)
	if err != nil {
		return err
	}
	if *orderBy != "" {
		*sortBy = *orderBy
	}

	iterations := *count
	if iterations < 0 {
		iterations = 0
	}

	i := 0
	for {
		if !*batch && utils.IsTerminal(os.Stdout) {
			fmt.Print("\033[H\033[2J")
		}
		// forward selected sorting flags to psCmd so batch-mode top keeps the
		// CPU/memory-oriented process table instead of ps -f's full-format view.
		psArgs := []string{"-sort", *sortBy}
		if *rev {
			psArgs = append(psArgs, "-r")
		}
		if *pids != "" {
			psArgs = append(psArgs, "-p", *pids)
		}
		if *users != "" {
			psArgs = append(psArgs, "-u", *users)
		}
		if *hideIdle {
			psArgs = append(psArgs, "-hide-idle")
		}
		if *fullCmd {
			psArgs = append(psArgs, "-ww")
		}
		if err := PsCmd(psArgs); err != nil {
			return err
		}
		i++
		if iterations != 0 && i >= iterations {
			break
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}

func parseTopDelay(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("invalid delay %q", value)
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil || seconds < 0 {
		return 0, fmt.Errorf("invalid delay %q", value)
	}
	return time.Duration(seconds * float64(time.Second)), nil
}
