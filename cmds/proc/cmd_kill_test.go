package proc

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillDryRunMatchesProcess(t *testing.T) {
	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-x", "sleep"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sleep") {
		t.Fatalf("unexpected kill dry-run output %q", out)
	}
	if !strings.Contains(out, strconv.Itoa(cmd.Process.Pid)) {
		t.Fatalf("expected dry-run output to include test child pid %d, got %q", cmd.Process.Pid, out)
	}
}

func TestKillCmdHelpUsesGroupedSections(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return KillCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("kill --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox kill [OPTION]... PID... | PATTERN", "Signals:", "Matching:", "-l, --list", "--dry-run"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestKillCmdOptionsListSignals(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "SIGTERM") || !strings.Contains(out, "SIGKILL") {
		t.Fatalf("unexpected signal list %q", out)
	}

}

// TestKillCmdListAllSignalsGridShape is a regression test for gobox kill -l
// previously listing only 5 hardcoded signals on a single line. It must now
// print the full 64-signal GNU grid, skipping the unused 32/33 slots.
func TestKillCmdListAllSignalsGridShape(t *testing.T) {
	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l"})
	})
	if err != nil {
		t.Fatal(err)
	}
	matches := regexp.MustCompile(`(\d+)\) (SIG\S+)`).FindAllStringSubmatch(out, -1)
	if len(matches) != 62 {
		t.Fatalf("expected 62 numbered signal entries (1-31, 34-64), got %d: %q", len(matches), out)
	}
	if strings.Contains(out, "32) ") || strings.Contains(out, "33) ") {
		t.Fatalf("expected signals 32/33 to be omitted, got %q", out)
	}
	if matches[0][1] != "1" || matches[0][2] != "SIGHUP" {
		t.Fatalf("expected first entry to be \"1) SIGHUP\", got %v", matches[0])
	}
	last := matches[len(matches)-1]
	if last[1] != "64" || last[2] != "SIGRTMAX" {
		t.Fatalf("expected last entry to be \"64) SIGRTMAX\", got %v", last)
	}
}

// TestKillCmdListRealtimeSignalNaming locks in GNU kill -l's real-time
// signal naming convention (verified against the live host's kill -l):
// 34-49 named relative to RTMIN, 50-64 named relative to RTMAX.
func TestKillCmdListRealtimeSignalNaming(t *testing.T) {
	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l"})
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"34) SIGRTMIN\t",
		"35) SIGRTMIN+1\t",
		"49) SIGRTMIN+15",
		"50) SIGRTMAX-14",
		"63) SIGRTMAX-1\t64) SIGRTMAX",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in kill -l output, got %q", want, out)
		}
	}
}

// TestKillCmdNumericRoundTripAllSignals verifies -l NAME <-> -l NUMBER
// round-trips work for signals beyond the original 5-signal table,
// including a real-time signal.
func TestKillCmdNumericRoundTripAllSignals(t *testing.T) {
	cases := []struct {
		name string
		num  string
	}{
		{"HUP", "1"},
		{"USR1", "10"},
		{"TERM", "15"},
		{"RTMIN", "34"},
		{"RTMAX", "64"},
	}
	for _, c := range cases {
		out, err := captureProcCmd(t, func() error { return KillCmd([]string{"-l", c.name}) })
		if err != nil {
			t.Fatalf("-l %s failed: %v", c.name, err)
		}
		if strings.TrimSpace(out) != c.num {
			t.Fatalf("-l %s: expected %q, got %q", c.name, c.num, strings.TrimSpace(out))
		}
		out, err = captureProcCmd(t, func() error { return KillCmd([]string{"-l", c.num}) })
		if err != nil {
			t.Fatalf("-l %s failed: %v", c.num, err)
		}
		if strings.TrimSpace(out) != c.name {
			t.Fatalf("-l %s: expected %q, got %q", c.num, c.name, strings.TrimSpace(out))
		}
	}
}

func TestKillCmdOptionsNumericSignalDryRunPid(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-9", "--dry-run", strconv.Itoa(os.Getpid())})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("unexpected dry-run pid output %q", out)
	}

}

func TestKillCmdOptionsSymbolicSignalKillsChild(t *testing.T) {

	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := KillCmd([]string{"-TERM", strconv.Itoa(cmd.Process.Pid)}); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("child did not exit after -TERM")
	}

}

func TestKillCmdOptionsListOneSignal(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l", "TERM"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "15" {
		t.Fatalf("unexpected single signal output %q", out)
	}

}

func TestKillCmdOptionsListNumericSignalName(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l", "15"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "TERM" {
		t.Fatalf("unexpected numeric signal output %q", out)
	}

}

func TestKillCmdOptionsFullMatchDryRun(t *testing.T) {

	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-f", "sleep 2"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, strconv.Itoa(cmd.Process.Pid)) || !strings.Contains(out, "sleep 2") {
		t.Fatalf("unexpected full match output %q", out)
	}

}

func TestKillCmdOptionsParentFilterDryRun(t *testing.T) {

	cmd := exec.Command("sleep", "2")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-P", strconv.Itoa(os.Getpid())})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, strconv.Itoa(cmd.Process.Pid)) {
		t.Fatalf("expected child pid in parent filter output %q", out)
	}

}

func TestKillCmdOptionsExactCommPartialMismatch(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-x", "slee"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "sleep") {
		t.Fatalf("expected partial comm mismatch, got %q", out)
	}

}

// TestSignalMatchesSkipsEPERMAndContinuesBatch is a regression test for
// pattern-matched kill (-f/-x/-P/-n/-o) aborting the whole batch on the
// first permission error. Native pkill skips a process it can't signal
// (EPERM) and keeps going; gobox previously returned immediately, leaving
// every later match in the batch un-signaled.
func TestSignalMatchesSkipsEPERMAndContinuesBatch(t *testing.T) {
	origKill := killSignal
	defer func() { killSignal = origKill }()

	var signaled []int
	killSignal = func(pid int, sig syscall.Signal) error {
		if pid == 111 {
			return syscall.EPERM
		}
		signaled = append(signaled, pid)
		return nil
	}

	matches := []procMatch{{pid: 111, cmd: "owned-by-someone-else"}, {pid: 222, cmd: "our-process"}}
	if err := signalMatches(matches, syscall.SIGTERM, false); err != nil {
		t.Fatalf("expected batch signal to succeed despite one EPERM match, got %v", err)
	}
	if len(signaled) != 1 || signaled[0] != 222 {
		t.Fatalf("expected only the non-EPERM match (pid 222) to be signaled, got %v", signaled)
	}
}

// TestSignalMatchesSkipsESRCHAndContinuesBatch mirrors the EPERM case for a
// process that exited between the /proc scan and the kill.
func TestSignalMatchesSkipsESRCHAndContinuesBatch(t *testing.T) {
	origKill := killSignal
	defer func() { killSignal = origKill }()

	var signaled []int
	killSignal = func(pid int, sig syscall.Signal) error {
		if pid == 111 {
			return syscall.ESRCH
		}
		signaled = append(signaled, pid)
		return nil
	}

	matches := []procMatch{{pid: 111, cmd: "already-exited"}, {pid: 222, cmd: "our-process"}}
	if err := signalMatches(matches, syscall.SIGTERM, false); err != nil {
		t.Fatalf("expected batch signal to succeed despite one ESRCH match, got %v", err)
	}
	if len(signaled) != 1 || signaled[0] != 222 {
		t.Fatalf("expected only the non-ESRCH match (pid 222) to be signaled, got %v", signaled)
	}
}

// TestSignalMatchesPropagatesOtherErrors ensures non-ESRCH/EPERM failures
// still abort the batch and surface to the caller.
func TestSignalMatchesPropagatesOtherErrors(t *testing.T) {
	origKill := killSignal
	defer func() { killSignal = origKill }()

	killSignal = func(pid int, sig syscall.Signal) error {
		return syscall.EINVAL
	}

	matches := []procMatch{{pid: 111, cmd: "x"}}
	if err := signalMatches(matches, syscall.SIGTERM, false); err != syscall.EINVAL {
		t.Fatalf("expected EINVAL to propagate, got %v", err)
	}
}

func TestKillCmdOptionsNewestAndOldestSelectOneProcess(t *testing.T) {

	token := "gobox-ut-new-old"
	cmd1 := exec.Command("sh", "-c", "sleep 3 & wait", token+"-1")
	if err := cmd1.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd1.Process.Kill()
		_, _ = cmd1.Process.Wait()
	}()
	time.Sleep(50 * time.Millisecond)
	cmd2 := exec.Command("sh", "-c", "sleep 3 & wait", token+"-2")
	if err := cmd2.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd2.Process.Kill()
		_, _ = cmd2.Process.Wait()
	}()

	newest, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-f", token, "-n"})
	})
	if err != nil {
		t.Fatal(err)
	}
	oldest, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"--dry-run", "-f", token, "-o"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.TrimSpace(newest), "\n") != 0 || strings.Count(strings.TrimSpace(oldest), "\n") != 0 {
		t.Fatalf("expected one pid from newest/oldest, newest=%q oldest=%q", newest, oldest)
	}
	if !strings.Contains(newest, strconv.Itoa(cmd2.Process.Pid)) || !strings.Contains(oldest, strconv.Itoa(cmd1.Process.Pid)) {
		t.Fatalf("unexpected newest/oldest selection, newest=%q oldest=%q", newest, oldest)
	}

}

func TestKillCmdOptionsInvalidPid(t *testing.T) {

	if err := KillCmd([]string{"not-a-pid"}); err == nil {
		t.Fatal("expected invalid pid error")
	}

}

// TestKillCmdOptionsNonexistentPidReturnsESRCH covers the direct
// syscall.Kill branch with a numerically-valid but nonexistent pid --
// previously only a non-numeric argument (which never reaches syscall.Kill
// at all) was tested, so a regression that swallowed the ESRCH error
// instead of propagating it would not have been caught.
func TestKillCmdOptionsNonexistentPidReturnsESRCH(t *testing.T) {
	pid := findUnusedPID(t)
	err := KillCmd([]string{strconv.Itoa(pid)})
	if err == nil {
		t.Fatalf("expected an error killing nonexistent pid %d, got success", pid)
	}
	if !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("expected ESRCH for nonexistent pid %d, got: %v", pid, err)
	}
}

func TestKillCmdOptionsMissingOperand(t *testing.T) {

	if err := KillCmd(nil); err == nil {
		t.Fatal("expected missing pid error")
	}

}

func TestKillCmdOptionsSignalOptionDryRun(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-s", "TERM", "--dry-run", strconv.Itoa(os.Getpid())})
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("unexpected -s dry-run output %q", out)
	}

}
