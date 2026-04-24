# Parity Skip Audit TODO

## High priority: can remove skip with test work only

- `tests/parity/net_parity_test.go:1092` `NP-012`
  - Current state: skipped with `linux only`.
  - Problem: `np -scan` parity case is a local TCP connect contract test and does not inherently depend on Linux-only `/proc` behavior.
  - Action: remove the Linux guard and make the case run wherever local TCP listen/dial is available.

- `tests/parity/planned_parity_test.go:438` `WATCH-002`
  - Current state: skipped because interval assertions may be flaky.
  - Problem: the case can be made stable with bounded-count or relative-count assertions instead of exact timing.
  - Action: add a watch cadence harness that compares refresh counts for different `-n` values within a bounded runtime window.

- `tests/parity/net_parity_test.go:265` `NETSTAT-020`
  - Current state: skipped because `netstat -c` is continuous.
  - Problem: current implementation already has a dedicated continuous loop path in `cmds/net/cmd_netstat.go`; parity is not covered.
  - Action: add a bounded subprocess/self-exec parity case that starts `netstat -c`, captures output for a short interval, then terminates and verifies multiple render cycles.

- `tests/parity/planned_parity_test.go:477` `KILL-002`
  - Current state: grouped under generic kill skips.
  - Problem: `kill -l` does not need a child-process fixture and can be tested now.
  - Action: add a structured parity case that validates common signals such as `HUP`, `INT`, `KILL`, `TERM`, plus `kill -l TERM` / `kill -l 15` parseability.

## Medium priority: remove skip after adding dedicated test fixtures

- `tests/parity/planned_parity_test.go:477` `KILL-001`
  - Current state: skipped due to missing controlled lifecycle fixture.
  - Action: add a controlled child process and verify default `TERM` behavior by observing process exit.

- `tests/parity/planned_parity_test.go:479` `KILL-003`
  - Current state: skipped due to missing controlled lifecycle fixture.
  - Action: add a signal-specific child fixture and verify `kill -s SIGNAL` behavior.

- `tests/parity/planned_parity_test.go:480` `KILL-004`
  - Current state: skipped due to missing controlled lifecycle fixture.
  - Action: reuse the same fixture to verify `kill -SIGNAL` short syntax.

- `tests/parity/planned_parity_test.go:481` `KILL-005`
  - Current state: skipped due to missing named-process fixture.
  - Action: add a deterministic `exec -a ... sleep` or symlinked executable fixture and verify `pkill -f`.

- `tests/parity/planned_parity_test.go:482` `KILL-006`
  - Current state: skipped due to missing named-process fixture.
  - Action: add the same fixture family and verify exact-name selection for `pkill -x`.

- `tests/parity/planned_parity_test.go:483` `KILL-007`
  - Current state: skipped due to missing parent-child tree fixture.
  - Action: build a parent process that spawns a child process and verify `pkill -P`.

- `tests/parity/planned_parity_test.go:484` `KILL-008`
  - Current state: skipped due to missing age-order fixture.
  - Action: create multiple same-pattern processes with controlled startup spacing and verify `pkill -n`.

- `tests/parity/planned_parity_test.go:485` `KILL-009`
  - Current state: skipped due to missing age-order fixture.
  - Action: reuse the same fixture and verify `pkill -o`.

## Implementation defects masked by current skips

- `tests/parity/planned_parity_test.go:323` `READPATH-007`
  - Current state: skipped as if this were only a distro-specific stderr normalization issue.
  - Actual problem: `cmds/fs/cmd_readpath.go` quiet mode suppresses stderr text but still returns a generic error, so gobox parity exit semantics are not yet aligned with `realpath -q`.
  - Action: align quiet-mode exit semantics first, then re-enable the parity case with normalized stderr assertions.

- `tests/parity/planned_parity_test.go:403` `TIMEOUT-003`
  - Current state: skipped for missing signal-ignoring fixture.
  - Actual problem: `cmds/proc/cmd_timeout.go` currently returns `124` even after the post-grace `KILL` path, which does not match native timeout kill-after semantics.
  - Action: fix `-k` exit behavior first, then add a signal-ignoring child fixture and parity case.

- `tests/parity/planned_parity_test.go:406` `TIMEOUT-004`
  - Current state: skipped for missing fixed-exit child fixture.
  - Actual problem: `cmds/proc/cmd_timeout.go` does not actually preserve child status when `--preserve-status` is set.
  - Action: fix preserve-status behavior first, then add a known-exit child fixture and parity case.

- `tests/parity/planned_parity_test.go:400` `TIMEOUT-002`
  - Current state: skipped for missing signal-trapping fixture.
  - Risk: after the timeout implementation fixes above, `-s SIGNAL` still needs explicit parity verification against a signal-trapping child.
  - Action: add a child process that traps `TERM`/custom signal and verify delivered-signal semantics.

## Reasonable environment-protection skips to keep

- Keep environment guards such as:
  - `linux only` on Linux-specific `/proc` and mountinfo cases
  - unix-like environment guard in `tests/parity/helpers_parity_test.go`
  - network precondition guards such as default IPv4 gateway unavailable and IPv6 loopback unavailable

## Environment baseline policy cleanup

- Native baseline commands in parity tests should be treated as required environment prerequisites, not as normal skip conditions.
  - Current code still contains native-missing skip paths for `curl`, `du`, `fio`, `lsof`, `md5sum`, `nc`, `ps`, plus generic helpers.
  - New policy: assume native commands exist; if missing, that indicates a broken parity environment.
  - Follow-up action: convert native-missing handling from "reasonable skip" to explicit environment precondition failure or centralized suite gating.

## Optional cleanup

- `tests/parity/net_parity_test.go:665` `NP-001`
  - Current state: skips if interface `lo` is not available by name.
  - Improvement: detect a loopback interface dynamically instead of hard-coding `lo`, to reduce unnecessary environment skips.
