package proc

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func WatchCmd(args []string) error {
	return WatchCmdWithContext(context.Background(), args)
}

func WatchCmdWithContext(ctx context.Context, args []string) error {
	fsFlags := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fsFlags.Float64("n", 2, "interval seconds")
	noTitle := fsFlags.Bool("t", false, "hide title")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox watch [-n SEC] [-t] COMMAND [ARG]...")
		fsFlags.PrintDefaults()
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
	for {
		if !*noTitle {
			fmt.Fprintf(os.Stdout, "Every %.1fs: %v\n\n", *interval, cmdArgs)
		}
		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		_ = cmd.Run()
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
	}
}
