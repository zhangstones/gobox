package proc

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"

	"gobox/cmds/utils"
)

func WatchCmd(args []string) error {
	return WatchCmdWithContext(context.Background(), args)
}

func WatchCmdWithContext(ctx context.Context, args []string) error {
	fsFlags := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fsFlags.Float64("n", 2, "interval seconds")
	noTitle := fsFlags.Bool("t", false, "hide title")
	appendMode := fsFlags.Bool("append", false, "append each refresh instead of clearing the screen")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox watch [-n SEC] [-t] [--append] COMMAND [ARG]...")
		fmt.Fprintln(os.Stderr, "Run a command periodically.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Behavior:")
		fmt.Fprintln(os.Stderr, "  default            refresh in-place by clearing the screen each iteration")
		fmt.Fprintln(os.Stderr, "  --append           keep prior output and append each iteration below it")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  -n SEC             interval in seconds between refreshes")
		fmt.Fprintln(os.Stderr, "  -t                 hide the title line")
		fmt.Fprintln(os.Stderr, "  --append           append output instead of clearing the screen")
		fmt.Fprintln(os.Stderr, "  -h, --help         show this help")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  gobox watch -n 1 date")
		fmt.Fprintln(os.Stderr, "  gobox watch --append -n 1 date")
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	cmdArgs := fsFlags.Args()
	if len(cmdArgs) == 0 {
		return fmt.Errorf("missing command")
	}
	delay := time.Duration(*interval * float64(time.Second))
	if delay <= 0 {
		delay = time.Second
	}
	clearScreen := !*appendMode && utils.IsTerminal(os.Stdout)
	for {
		if clearScreen {
			fmt.Fprint(os.Stdout, "\033[H\033[J")
		}
		if !*noTitle {
			fmt.Fprintf(os.Stdout, "Every %.1fs: %v\n\n", *interval, cmdArgs)
		}
		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "watch: %v\n", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
	}
}
