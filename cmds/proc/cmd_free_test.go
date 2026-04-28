package proc

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestFreeProducesMemoryRows(t *testing.T) {
	out, err := captureProcCmd(t, func() error {
		return FreeCmd([]string{"-m"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Mem:") {
		t.Fatalf("unexpected free output %q", out)
	}
}

func TestFreeCmdHelpUsesGroupedSections(t *testing.T) {
	out, err := captureProcOutput(t, func() error {
		return FreeCmd([]string{"--help"})
	})
	if err != nil {
		t.Fatalf("free --help failed: %v", err)
	}
	for _, want := range []string{"Usage: gobox free [OPTION]...", "Units:", "Sampling:", "-s SEC", "-c COUNT"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected help to contain %q, got %q", want, out)
		}
	}
}

func TestFreeDefault(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd(nil) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"total", "Mem:", "Swap:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in free output %q", want, out)
		}
	}
}

func TestFreeHuman(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Mem:", "Swap:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in free output %q", want, out)
		}
	}
}

func TestFreeMiB(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-m"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Mem:") {
		t.Fatalf("expected MiB output, got %q", out)
	}
}

func TestFreeGiB(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-g"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Mem:") {
		t.Fatalf("expected GiB output, got %q", out)
	}
}

func TestFreeCount(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-s", "0", "-c", "2"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "Mem:") != 2 {
		t.Fatalf("expected two samples, got %q", out)
	}
}

func TestFreeNonpositiveCountCoercesToOneSample(t *testing.T) {
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-c", "0"}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "Mem:") != 1 {
		t.Fatalf("expected one sample, got %q", out)
	}
}

func TestFreeInvalidIntervalArgument(t *testing.T) {
	if _, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-s", "bad"}) }); err == nil {
		t.Fatal("expected invalid interval flag error")
	}
}

func TestFreeInvalidCountArgument(t *testing.T) {
	if _, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-c", "bad"}) }); err == nil {
		t.Fatal("expected invalid count flag error")
	}
}

func setupFreeInjected(t *testing.T) {
	t.Helper()
	oldReadMemInfo, oldSleep := readMemInfoData, freeSleep
	t.Cleanup(func() { readMemInfoData, freeSleep = oldReadMemInfo, oldSleep })
}

func TestFreeReadErrorReturnsError(t *testing.T) {
	setupFreeInjected(t)
	readMemInfoData = func() (map[string]uint64, error) { return nil, os.ErrPermission }
	if _, err := captureProcCmd(t, func() error { return FreeCmd(nil) }); err == nil {
		t.Fatal("expected meminfo read error")
	}
}

func TestFreeZeroSwapAndSmallMiBValuesAreStable(t *testing.T) {
	setupFreeInjected(t)
	readMemInfoData = func() (map[string]uint64, error) {
		return map[string]uint64{
			"MemTotal":     512 * 1024,
			"MemFree":      128 * 1024,
			"Buffers":      64 * 1024,
			"Cached":       64 * 1024,
			"SReclaimable": 0,
			"MemAvailable": 256 * 1024,
			"SwapTotal":    0,
			"SwapFree":     0,
		}, nil
	}
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-m"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Mem:") || !strings.Contains(out, "Swap:") || !strings.Contains(out, "           0") {
		t.Fatalf("unexpected injected free output %q", out)
	}
}

func TestFreeHumanAndGiBUnitsAreStable(t *testing.T) {
	setupFreeInjected(t)
	readMemInfoData = func() (map[string]uint64, error) {
		return map[string]uint64{
			"MemTotal":     2 * 1024 * 1024 * 1024,
			"MemFree":      1 * 1024 * 1024 * 1024,
			"MemAvailable": 1 * 1024 * 1024 * 1024,
		}, nil
	}
	human, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-h"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(human, "2.0G") {
		t.Fatalf("expected human GiB output, got %q", human)
	}
	gib, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-g"}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gib, "           2") {
		t.Fatalf("expected GiB numeric output, got %q", gib)
	}
}

func TestFreeCountUsesInjectedSleepBetweenSamples(t *testing.T) {
	setupFreeInjected(t)
	reads := 0
	sleeps := 0
	readMemInfoData = func() (map[string]uint64, error) {
		reads++
		return map[string]uint64{"MemTotal": 1024, "MemFree": 512, "MemAvailable": 512}, nil
	}
	freeSleep = func(time.Duration) { sleeps++ }
	out, err := captureProcCmd(t, func() error { return FreeCmd([]string{"-s", "1", "-c", "3"}) })
	if err != nil {
		t.Fatal(err)
	}
	if reads != 3 || sleeps != 2 || strings.Count(out, "Mem:") != 3 {
		t.Fatalf("expected 3 reads, 2 sleeps, and 3 samples; reads=%d sleeps=%d out=%q", reads, sleeps, out)
	}
}

func TestParseMemInfo(t *testing.T) {
	mem, err := parseMemInfo(strings.NewReader("MemTotal: 2 kB\nBad:\nCached: nope kB\nSwapFree: 1 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if mem["MemTotal"] != 2048 || mem["SwapFree"] != 1024 {
		t.Fatalf("unexpected parsed meminfo %#v", mem)
	}
	if _, ok := mem["Cached"]; ok {
		t.Fatalf("invalid numeric field should be ignored: %#v", mem)
	}
}
