package proc

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
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

func TestKillCmdOptionsListSignals(t *testing.T) {

	out, err := captureProcCmd(t, func() error {
		return KillCmd([]string{"-l"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "TERM") || !strings.Contains(out, "KILL") {
		t.Fatalf("unexpected signal list %q", out)
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
	if strings.TrimSpace(out) != "terminated" && strings.TrimSpace(out) != "15" {
		t.Fatalf("unexpected single signal output %q", out)
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
