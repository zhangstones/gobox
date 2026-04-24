# Command Enhancement Status

## Completed

- [x] Remove `cmp` and add `diff`
  - [x] Removed `cmp` from command registration, README command list, command design docs, parity tests, smoke tests, and related test cases.
  - [x] Retired `cmds/text/cmd_cmp.go` and `cmds/text/cmd_cmp_test.go`.
  - [x] Added `diff` registration and implementation in `cmds/text/cmd_diff.go`.
  - [x] Added focused unit tests in `cmds/text/cmd_diff_test.go`.
  - [x] Added `diff` to README, `docs/CMD-DESIGN.md`, `docs/TEST-CASES.md`, and parity helpers.
  - [x] Supported usage: `gobox diff FILE1 FILE2`.
  - [x] Supported parameters: `-u`, `--unified`, `-q`, `--brief`, `-r`, `--recursive`, `-N`, `--new-file`, `--strip-trailing-cr`.
  - [x] Supported normal text diff output for changed, added, and deleted line ranges.
  - [x] Supported unified diff output for review and patch-style inspection.
  - [x] Supported brief mode, recursive directory comparison, missing files as empty files with `-N`, CRLF/LF tolerance, stdin operand `-`, binary file reporting, and diff-style exit code semantics.
  - [x] Added unit and native parity coverage for equal files, changed files, insertions, deletions, unified output, brief output, recursive directories, missing files with `-N`, CRLF stripping, stdin, and binary detection.

- [x] Enhance `du` command parameters
  - [x] Supported parameters: `-h`, `-s`, `-a`, `-c`, `-d`, `--max-depth`, `--exclude`, `-x`, `--apparent-size`.
  - [x] Supported human-readable output, summary-only output, all-file output, total output, max-depth traversal, exclude filtering, single-filesystem traversal, and apparent-size calculation.
  - [x] Documented compatibility in `docs/CMD-DESIGN.md` and case coverage in `docs/TEST-CASES.md`.
  - [x] Added targeted unit tests and native-backed parity tests.

- [x] Enhance `df` command parameters
  - [x] Supported parameters: `-h`, `-H`, `-T`, `-i`, `-a`, `-l`, `-t`, `-x`, `--total`, `-P`.
  - [x] Supported human-readable output, SI units, filesystem type output, inode output, all filesystems, local-only filtering, type include/exclude filters, total summary, and POSIX-compatible output.
  - [x] Preserved injectable `readMounts`, `statDfPath`, and `statfsDfPath` seams for deterministic unit tests.
  - [x] Documented compatibility in `docs/CMD-DESIGN.md` and case coverage in `docs/TEST-CASES.md`.
  - [x] Added targeted unit tests and native-backed parity tests.

- [x] Enhance `ps` command parameters
  - [x] Supported parameters: `-e`, `-A`, `-f`, `-F`, `-l`, `-u`, `-p`, `-C`, `-o`, `--sort`, `-ww`, `aux`.
  - [x] Supported listing all processes, full/long output formats, user/PID/command-name filters, custom output columns, selected-field sorting, wide output, and common BSD-style `ps aux`.
  - [x] Preserved gobox extensions: `-sort`, `-r`, `-n`, `-full`, `-comm`, `-i`.
  - [x] Added GNU-compatible field aliases including `%cpu`, `%mem`, `args`, `comm`, `stat`, `etime`, `time`, `rss`, and `vsz`.
  - [x] Added targeted unit tests and native-backed parity tests for the new parameters.

- [x] Enhance `top` command parameters
  - [x] Supported parameters: `-b`, `-n`, `-d`, `-p`, `-u`, `-H`, `-i`, `-c`, `-o`.
  - [x] Supported batch output, bounded iterations, fractional/integer delay parsing, PID/user filtering, accepted thread mode, idle filtering, full command line display, and selected-field sorting through `PsCmd`.
  - [x] Documented `-H` as process-level compatible rather than full thread listing.
  - [x] Added targeted unit tests and native-backed parity tests for the new parameters.

- [x] Enhance `netstat` command parameters
  - [x] Supported parameters: `-a`, `-t`, `-u`, `-l`, `-n`, `-p`, `-r`, `-i`, `-s`, `-c`, `-e`.
  - [x] Also supported common related flags: `-x`, `-4`, `-6`, `-o`, `-W`, long aliases, and combined short flags such as `-tnlp`.
  - [x] Supported all sockets, TCP/UDP/Unix filtering, listening sockets, numeric output, PID/program output when requested, route table output, interface output, protocol statistics, continuous refresh, and extended/timer information.
  - [x] Documented numeric-only behavior and partial compatibility areas in `docs/CMD-DESIGN.md`.
  - [x] Added targeted unit tests and native-backed parity tests for the new parameters.

## Testing Policy

- Unit tests cover command-level behavior directly and do not rely on parity tests for correctness.
- Parity tests compare gobox behavior against native commands where a native baseline is meaningful and stable.
- Smoke tests intentionally cover only a small number of representative basic usage scenarios.

## Deferred

- [ ] Enhance `iostat` command parameters
  - [ ] Support parameters: `-x`, `-d`, `-c`, `-k`, `-m`, `-t`, `-y`, `-p`, `-N`, `-z`.
  - [ ] Support extended device statistics, disk-only output, CPU-only output, KB/s and MB/s units, timestamp output, skipping the first report since boot, device partition output, device mapper names, and hiding inactive devices.
  - [ ] Decide whether to keep current cgroup-based output as gobox-specific mode or switch default data source to `/proc/diskstats` for closer Linux `iostat` compatibility.
  - [ ] Add positional syntax support: `iostat [options] [interval [count]]`, without breaking existing `-i` and `-n` behavior.
  - [ ] Calculate extended fields such as `r/s`, `w/s`, `rkB/s`, `wkB/s`, `await`, `aqu-sz`, and `%util` from diskstats deltas.
  - [ ] Implement `-c` and `-d` as independent CPU/disk section selectors, with CPU stats from `/proc/stat`.
  - [ ] Implement `-y`, `-p [DEVICE|ALL]`, and `-N`.
  - [ ] Update `cmds/disk/cmd_iostat.go`, add targeted unit tests, add disk parity tests, and update `docs/CMD-DESIGN.md` / `docs/TEST-CASES.md`.
