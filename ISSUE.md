# Code Review Issues - New Commands (2026-04-18)

## Summary

Reviewed 7 new commands: ioperf, ifstat, np, md5sum, rand, seq, hping

---

## Issues Found and Fixed

### 1. File Handle Leak in md5sumCheck (FIXED)
- **File**: cmds/disk/cmd_md5sum.go
- **Lines**: 115-228 (md5sumCheck function)
- **Severity**: Medium
- **Description**: The file `f` opened at line 119 was only closed at line 221, after the scanner loop. If any `continue` statement was hit inside the scanner loop (lines 149, 157, 170), the file handle was leaked.
- **Fix Applied**: Added `defer f.Close()` immediately after successfully opening the file at line 127, and removed the explicit close at the end of the loop.

### 2. Goroutine Leak in npProgressReporter (FIXED)
- **File**: cmds/net/cmd_np.go
- **Lines**: 534-549 (npProgressReporter function)
- **Severity**: Medium
- **Description**: The progress reporter goroutine runs in an infinite loop with `ticker := time.NewTicker(time.Second)` and had no way to stop. When `npTCP` returns, this goroutine would continue running indefinitely.
- **Fix Applied**: 
  - Added `stopChan` parameter to `npProgressReporter` function
  - Added case to select statement to receive from stopChan and exit

---

## Issues Not Bugs (False Positives)

The following were initially thought to be bugs but are actually correct:

1. **Race condition in npTCPWorker** - The mutex IS properly used to protect the latencies slice access at lines 249-251.

2. **Race condition in npUDP** - The mutex IS properly used to protect the latencies slice access at lines 303-305.

3. **Inverted logic in hpingFIN** - The logic is actually correct. The `received` variable tracks "RST received (port closed)" and `lost` tracks "no response (port open|filtered)". The naming is just confusing but the behavior is correct.

4. **seq.go combined flag -sSEP** - The combined flag handling works correctly as verified by tests.

---

## Pre-existing Test Bugs (Not Related to New Commands)

1. **cmd_sed_test.go** - Uses undefined variable `filename` instead of `filepath` (lines 537, 539, 545, 556, 572, 577, 587)
2. **cmd_np_test.go TestParsePortRangeMixedCommaAndDash** - Test says "expected 7 ports" but expected array has 8 elements

---

## Test Results

All tests for new commands pass:
- md5sum tests: PASS
- ioperf tests: PASS  
- rand tests: PASS
- seq tests: PASS
- np tests: PASS
- hping tests: PASS
- ifstat tests: PASS
