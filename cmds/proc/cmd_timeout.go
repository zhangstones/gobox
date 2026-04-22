package proc

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type timeoutExitError int

func (e timeoutExitError) Error() string { return fmt.Sprintf("exit code %d", int(e)) }
func (e timeoutExitError) ExitCode() int { return int(e) }

func TimeoutCmd(args []string) error {
	fsFlags := flag.NewFlagSet("timeout", flag.ContinueOnError)
	sigName := fsFlags.String("s", "TERM", "signal to send on timeout")
	fsFlags.StringVar(sigName, "signal", "TERM", "signal to send on timeout")
	killAfter := fsFlags.String("k", "", "send KILL after duration")
	preserve := fsFlags.Bool("preserve-status", false, "preserve command exit status")
	fsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox timeout [OPTION] DURATION COMMAND [ARG]...")
		fsFlags.PrintDefaults()
	}
	if err := fsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	rest := fsFlags.Args()
	if len(rest) < 2 {
		return fmt.Errorf("missing duration or command")
	}
	d, err := parseDurationArg(rest[0])
	if err != nil {
		return err
	}
	sig, err := parseSignal(*sigName)
	if err != nil {
		return err
	}
	var killAfterDuration time.Duration
	if *killAfter != "" {
		killAfterDuration, err = parseDurationArg(*killAfter)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command(rest[1], rest[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case err := <-done:
		return commandExitErr(err)
	case <-timer.C:
		if cmd.Process != nil {
			_ = cmd.Process.Signal(sig)
			if *killAfter != "" {
				killTimer := time.NewTimer(killAfterDuration)
				defer killTimer.Stop()
				select {
				case <-done:
					return timeoutExitError(124)
				case <-killTimer.C:
					_ = cmd.Process.Kill()
				}
			}
		}
		<-done
		if *preserve {
			return timeoutExitError(124)
		}
		return timeoutExitError(124)
	}
}

func parseDurationArg(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	return time.Duration(f * float64(time.Second)), nil
}

func parseSignal(s string) (os.Signal, error) {
	s = strings.TrimPrefix(strings.ToUpper(s), "SIG")
	switch s {
	case "TERM", "15":
		return syscall.SIGTERM, nil
	case "KILL", "9":
		return syscall.SIGKILL, nil
	case "INT", "2":
		return syscall.SIGINT, nil
	case "HUP", "1":
		return syscall.SIGHUP, nil
	default:
		return nil, fmt.Errorf("unsupported signal %s", s)
	}
}

func commandExitErr(err error) error {
	if err == nil {
		return nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return timeoutExitError(ee.ExitCode())
	}
	return err
}
