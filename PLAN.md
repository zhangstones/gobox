# Command Enhancement Status

## Current Execution Plan

### Scope

- Focus area: command-parameter semantics, parity-test strength, and documentation consistency
- Goal: eliminate remaining parameter-semantic drift across high-risk commands and replace weak parity assertions with structure/behavior assertions.
- Command batches:
  - Batch A: `ps`, `top`
  - Batch B: `netstat`, `ifstat`, `iostat`
  - Batch C: `lsof`, `free`, other proc/system commands that still rely on weak parity checks

### Global Rules

- Any parity case that only checks keyword existence is a weak case and must be upgraded or removed.
- Every parameter under cleanup must be described consistently in:
  - `docs/CMD-DESIGN.md`
  - implementation behavior
  - `docs/CMD-SPECS.md`
  - `docs/TEST-CASES.md`
  - unit tests and parity tests
- Cleanup order for each command:
  - inventory weak cases and semantic drift
  - strengthen tests and documentation
  - fix implementation drift exposed by stronger tests
  - re-run focused validation

### Stage 0: Long-Plan Setup

- Status: completed
- Done:
  - Expand the work from `ps` only to a repo-wide high-risk command cleanup plan.
  - Split work into command batches to avoid mixing unrelated semantics in one pass.

### Stage 1: Baseline Inventory

- Status: completed
- Findings:
  - `tests/parity/proc_parity_test.go` still contains outdated assumptions that `ps` default and `ps -e` share the same PID view.
  - `PS-011`, `PS-012`, `PS-014`, `PS-015`, and `PS-016` still rely mainly on substring existence instead of row-set or column-shape assertions.
  - `PS-018` to `PS-021` are inconsistent with `docs/TEST-CASES.md`; docs only retain `PS-018`, but parity still keeps `PS-019`, `PS-020`, and `PS-021`.
  - `PS-018` only checks that `aux` exposes a few header keywords; it does not verify the individual semantics of BSD `a`, `x`, and `u`.
  - `PS-009` only checks that `-ww` prints a command column; it does not prove that width truncation is actually disabled.
  - `top` parity still has several weak keyword-only checks and currently validates less than the new interactive/runtime behavior deserves.
  - `netstat`, `ifstat`, and `iostat` have a history of “keyword exists” style parity checks and need the same audit method applied.
  - `lsof` and `free` still include some weak cases that should be upgraded after the proc/network batch is stable.

### Stage 2: Batch A Cleanup (`ps`, `top`)

- Status: completed
- Done:
  - Rewrote weak `ps` parity cases so they verify PID sets, header shapes, column semantics, width behavior, and BSD selection semantics instead of relying on substring existence.
  - Collapsed BSD `ps` parity coverage back into `PS-018`-scoped assertions so parity coverage is aligned with `docs/TEST-CASES.md`.
  - Strengthened `top` batch parity assertions to verify summary presence, table location, `-p` PID filtering, `-u` user filtering, `-i` row-count behavior, and `-o PID` ordering.
  - Fixed `ps ax` implementation drift so BSD non-`u` mode now uses a native-like `PID TTY STAT TIME COMMAND` shape instead of the gobox default table.
- Validation:
  - `go test ./tests/parity -run 'TestParity_(PsCases|TopCases)$' -count=1`
  - `go test ./cmds/proc -count=1`

### Stage 3: Batch B Cleanup (`netstat`, `ifstat`, `iostat`)

- Status: completed
- Todo:
  - Inventory weak parity cases and stale assumptions for network/stat commands.
  - Strengthen parity assertions from keyword existence to row-set, field-shape, or behavior assertions.
  - Fix implementation drift exposed by stronger tests.
  - Sync command docs and case docs.
- Inventory findings:
  - `tests/parity/net_parity_test.go` still contains many keyword-existence assertions for `netstat`, especially around `-l`, `-t`, `-u`, `-x`, `-e`, `-o`, `-r`, `-i`, and `-s`.
  - `NETSTAT-005` and `NETSTAT-006` still encode gobox-specific assumptions that `-n` and `-a` are no-op relative to the default view; these need explicit re-validation instead of being carried forward as doctrine.
  - `ifstat` parity currently checks that headers or option-specific column names exist, but does not yet validate row shape, repeated-sample count, or interface filtering rigorously.
  - `iostat` parity has a few structured checks already, but `-H`, `--cgroup`, and `--help` are still mostly keyword-based and need stronger contract assertions.
  - Batch priority inside Stage 3:
    - first: `netstat`
    - second: `ifstat`
    - third: `iostat`
- Progress:
  - Strengthened `netstat` parity for `-l`, `-t`, `-u`, `-x`, `-e`, and `-o` so the tests now validate row protocol, filtered-row retention, and extended/timer column shape instead of relying only on keyword presence.
  - Strengthened `ifstat` parity for `-A`, `-d`, `-e`, `-i`, `-n`, and `-p` so the tests now validate header/row shape and repeated-sample behavior instead of only checking keyword presence.
  - Strengthened `iostat` contract checks for `-H`, `--cgroup`, and `--help` so the tests now validate header shape, structured rows, and grouped help content.
  - Validation checkpoint:
    - `go test ./tests/parity -run 'TestParity_NetstatCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_IfstatCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_IostatCases$' -count=1`
    - `go test ./cmds/disk -count=1`

### Stage 4: Batch C Cleanup (`lsof`, `free`, remaining proc/system commands`)

- Status: completed
- Todo:
  - Audit remaining parity suites for weak keyword-only assertions.
  - Upgrade or remove weak cases.
  - Fix any implementation drift exposed by the stronger coverage.
  - Sync docs where semantics were previously overstated or underspecified.
- Inventory findings:
  - `lsof` still has several weak parity cases that mainly assert substring presence, especially the default view, `-c`, `-i`, `-iTCP`, `-iUDP`, `-i :PORT`, and `FILE...` filtering.
  - `free` is in better shape structurally, but `-h`, `-m`, and `-g` still lean on coarse output-shape checks and can be tightened further.
- Batch priority inside Stage 4:
  - first: `lsof`
  - second: `free`
- Progress:
  - Strengthened `lsof` parity for the default view, `-c`, `-i`, `-iTCP`, `-iUDP`, `-i :PORT`, and `FILE...` so tests now validate row filtering instead of only checking substring presence.
  - Tightened `free` parity for `-h`, `-m`, and `-g` so tests now verify unit and numeric-row semantics more explicitly.
  - Audited `timeout`, `watch`, and `kill`; they are primarily behavior-driven already and did not expose the same weak keyword-only parity pattern as the earlier command groups.
  - Validation checkpoint:
    - `go test ./tests/parity -run 'TestParity_(LsofCases|FreeCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(TimeoutCases|WatchCases|KillCases)$' -count=1`
    - `go test ./cmds/proc -count=1`

### Stage 5: Final Validation

- Status: completed
- Todo:
  - Run focused unit and parity validation after each batch.
  - Run the relevant smoke tests when command behavior changes affect end-to-end CLI behavior.
  - Run `go test ./cmds/proc -count=1`
  - Run `go test ./tests/parity -run 'TestParity_PsCases$' -count=1`
  - Update this file after each batch with completion notes, remaining drift, and next-batch entry criteria.
- Progress:
  - Focused validations completed for:
    - `ps/top`
    - `netstat/ifstat/iostat`
    - `lsof/free`
    - `timeout/watch/kill`
    - `df/stat/head/diff/tail`
  - Cross-batch parity checkpoint passed:
    - `go test ./tests/parity -run 'TestParity_(PsCases|TopCases|NetstatCases|IfstatCases|IostatCases|LsofCases|FreeCases|TimeoutCases|WatchCases|KillCases)$' -count=1`
  - Smoke checkpoint passed:
    - `go test ./tests/smoke -count=1`
  - Additional fs/text parity weak-case audit completed:
    - strengthened `diff -r` from keyword checks to recursive line-set parity
    - tightened `tail` follow-mode cases so `-n 0` no longer passes on replayed baseline content
    - tightened `stat` parity to validate labeled fields instead of generic keyword existence
    - tightened `iostat --help` so parity now validates grouped help section ordering instead of just keyword presence
    - tightened `ip addr` / `ip link` so parity now validates per-interface block structure instead of only checking a few keywords
    - tightened `curl --bench` / `nc --bench` so parity now validates parsed summary lines instead of just checking that one keyword appears somewhere
    - tightened `dig +answer` / `np` summary cases so parity now validates answer-row and summary-line structure instead of only checking bare substrings
    - tightened `top` / `free` summary checks so parity now validates summary-line placement, process-header structure, and numeric `Mem:` rows instead of only checking coarse keywords
    - tightened `lsof` default/network cases and `ioperf/fio` read-write summaries so parity now validates header/row structure and section lines rather than scanning the whole output for loose keywords
    - tightened `md5sum/sha256sum --warn` and a few remaining `ps` header checks so parity now prefers line-level assertions over whole-output substring checks
    - tightened remaining `iostat/ioperf` detail checks so parity now prefers header/section-line assertions for units, block size, and percentile output
    - tightened a remaining `top` variant guard so parity now requires a real process-table header instead of only checking for a loose `PID` substring
    - tightened `netstat -r/-i` and a few `df` header checks so parity now validates concrete header fields instead of only checking broad substrings
    - tightened a few remaining `lsof/free` line checks so parity now prefers row-level presence over scanning the entire output blob
  - Additional implementation drift fixed:
    - `stat -f` now renders common filesystem type names such as `xfs` instead of always emitting the raw magic number
  - Additional validation checkpoints passed:
    - `go test ./cmds/fs -count=1`
    - `go test ./cmds/net -count=1`
    - `go test ./tests/parity -run 'TestParity_(DfCases|StatCases|HeadCases|DiffCases|TailCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_LsofCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_IpCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(CurlCases|NcCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(DnsCases|NpCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(NetstatCases|IfstatCases|IpCases|CurlCases|NcCases|DnsCases|NpCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(TopCases|FreeCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(LsofCases|IoperfFioCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_FreeCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(Md5sumCases|Sha256sumCases|PsCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(IostatCases|IoperfFioCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_TopCases$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(NetstatCases|DfCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(LsofCases|FreeCases)$' -count=1`
    - `go test ./tests/parity -run 'TestParity_(PsCases|TopCases|NetstatCases|IfstatCases|IpCases|CurlCases|NcCases|DnsCases|NpCases|LsofCases|FreeCases|DfCases|StatCases|HeadCases|DiffCases|TailCases|IostatCases|IoperfFioCases)$' -count=1`
    - `go test ./cmds/fs ./cmds/proc ./cmds/disk -count=1`
    - `go test ./cmds/proc ./cmds/disk -count=1`
    - `go test ./tests/parity -run 'TestParity_(PsCases|TopCases|NetstatCases|IfstatCases|IostatCases|LsofCases|FreeCases|TimeoutCases|WatchCases|KillCases|DfCases|StatCases|HeadCases|DiffCases|TailCases)$' -count=1`
    - `go test ./cmds/fs ./cmds/proc ./cmds/net ./cmds/disk -count=1`
  - Follow-up correction:
    - `LSOF-004` parity was tightened too far and incorrectly treated `UDP` rows as leaked output for `lsof -i`; this was corrected to match native `lsof -i` semantics, which include both TCP and UDP network rows.
  - Final sweep completed:
    - strengthened remaining weak `netstat`/`ps`/`iostat` parity checks into row/header-shape assertions where stable, and relaxed a few over-strict `df`/`top` cases back to behavior-based assertions where environment-specific native output makes exact set equality brittle.
    - Validation passed with writable Go caches under `/tmp`:
      - `go test ./tests/parity -run 'TestParity_(PsCases|TopCases|NetstatCases|IfstatCases|IpCases|CurlCases|NcCases|DnsCases|NpCases|LsofCases|FreeCases|DfCases|StatCases|HeadCases|DiffCases|TailCases|IostatCases|IoperfFioCases)$' -count=1`
      - `go test ./cmds/fs ./cmds/proc ./cmds/disk -count=1`
      - `go test ./cmds/net -count=1`
      - `go test ./tests/smoke -count=1`
