# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Test

```bash
go build .              # Build main binary (outputs: gobox)
go build -o <name> .    # Build with custom name
go test ./...           # Run all tests
go test -v <file>       # Run single test file
go test ./tests/smoke   # Run smoke tests only
```

## Architecture

gobox is a single-binary utility toolkit inspired by BusyBox. A single executable dispatches to different subcommands from a shared registry.

### Command Implementation Pattern

- Each command lives in its own file: `cmd_<name>.go`, `cmd_find.go`, `cmd_du.go`, `cmd_ps.go`, etc.
- Each file exports a function like `func <name>Cmd(args []string) error`
- Commands are registered through package `init()` into `cmds/base/command.go`
- Each functional area keeps its own registration file such as `cmds/fs/register.go`, `cmds/net/register.go`
- `main.go` looks up the command from the registry instead of maintaining a large switch table

### Command Signature Convention

All command functions follow this pattern:
```go
func <name>Cmd(args []string) error
```

Return `nil` on success, error on failure. Exit codes are handled by main.go:
- Exit 0: help/version
- Exit 1: missing command / usage error
- Exit 2: command execution error
- Exit 127: unknown command

### Unit Tests

Every new command MUST have comprehensive test coverage in `cmd_<name>_test.go`:
- Normal cases: standard inputs and expected outputs
- Edge cases: empty files, single line, very long lines, special characters
- Error cases: non-existent files, permission denied, invalid arguments
- Bug fixes MUST include regression test cases to prevent the bug from recurring

**Test execution**: Unit tests MUST use direct function calls instead of `exec.Command`. Use stdout/stderr redirection via `os.Pipe()` to capture output. This ensures tests are fast, reliable, and don't depend on the compiled binary being present.

Example of correct approach:
```go
// Helper to run command and capture output
func runCmd(args []string) (string, error) {
    var buf bytes.Buffer
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    err := MyCmd(args)
    w.Close()
    io.Copy(&buf, r)
    os.Stdout = old
    return buf.String(), err
}

// In test:
output, err := runCmd([]string{"arg1", "arg2"})
```

Example of prohibited approach:
```go
// DO NOT use exec.Command in tests
cmd := exec.Command("./gobox", "mycmd", "arg1")
output, err := cmd.Output()
```

### Smoke Tests

Smoke tests (`tests/smoke/`) provide a quick sanity check that every command can run without crashing. They cover the 1-3 most core features of each command:
- Cross-platform examples: `find`, `du`, `xargs`, `grep`, `sed`
- Network examples: `dig`, `curl`, `nc`, `tw`
- System examples: `ps`, `top`, `iostat`, `netstat`

Run smoke tests before submitting PRs:
```bash
go test ./tests/smoke -v
```

### Parity Tests

Parity tests (`tests/parity/`) verify compatibility against native commands or enforce stable gobox-only behavior where no native baseline exists.

- Use parity tests for user-visible behavior that should stay aligned with native tools
- Prefer exact parity for stable text commands, structured parity for dynamic table output, and contract tests for gobox extensions
- Every documented case in `docs/TEST-CASES.md` should map to an explicit parity or contract test, or be skipped with a clear environment reason
- Keep parity helpers and case IDs in `tests/parity/`, rather than mixing parity logic into command package unit tests

Code organization requirements under `tests/parity/`:
- Put shared runners, normalizers, temp-env helpers, and local server/socket helpers in `tests/parity/helpers_parity_test.go`
- Group cases by domain in dedicated files such as `fs_parity_test.go`, `text_parity_test.go`, `net_parity_test.go`, `proc_parity_test.go`, `disk_parity_test.go`
- Keep test names and subtest IDs traceable to `docs/TEST-CASES.md` case IDs
- Put setup, normalization, and assertions next to the case matrix in the same parity file unless they are broadly reusable
- Do not move parity-only helpers back into `cmds/*` package tests; unit tests and parity tests serve different purposes

Useful commands:
```bash
go test ./tests/parity -v
go test ./tests/parity -run TestParity_IostatCases -v
```

### Documentation Requirements

When adding new features, changing behavior, or fixing bugs that affect user-visible functionality:
1. Update README.md with new command documentation and usage examples
2. Update the command's help text (typically in the command file)
3. Update the relevant docs under `docs/`
4. Ensure help text, README, and docs are consistent with each other

Recommended docs under `docs/`:
- `docs/CMD-DESIGN.md`: command support matrix, compatibility level (`✅/⚠️/🆕`), and intended user-visible behavior. Read this first when changing command flags or semantics.
- `docs/TEST-DESIGN.md`: testing strategy and layering guidance for unit, parity, and smoke tests. Read this when deciding how a new behavior should be validated.
- `docs/TEST-CASES.md`: case-first coverage matrix that maps command features to concrete test IDs. Update this alongside new behavior and make test names traceable back to case IDs.

### Design Principles

**Less is more**: Only implement the minimum necessary set of commands and parameters. Do not add features "just in case". Every addition must be justified by a real K8s debugging scenario.

**Minimum changes principle**: When modifying code or fixing bugs, use the minimum changes necessary to achieve the goal. Prefer:
- Targeted fixes over refactoring
- Reusing existing test infrastructure over creating new tests
- Running specific tests (`-run` flag) over full test suites when validating
- Simple solutions over complex ones

**Failure recovery principle**: When a task or subagent fails:
1. Verify the actual problem state before attempting fixes
2. Retry with improved approach if initial attempt fails
3. Try alternative methods if one approach doesn't work
4. Only escalate as unresolved if all reasonable attempts are exhausted
5. **Never ignore failures** - always verify and attempt resolution

**When adding commands**:
- Implement only the most commonly used parameters
- Avoid completeness (e.g., awk can replace cut, so don't add cut)
- Avoid redundant commands that kubectl exec can handle (e.g., kill, timeout)
- Question every new addition: "Is this truly necessary?"

### Shared Utilities

- `cmds/utils/utils.go` - Common helpers such as terminal detection and size formatting
- Tests live alongside implementation: `cmd_foo.go` → `cmd_foo_test.go`

### Dependencies

- `github.com/mitchellh/go-ps` - Cross-platform process listing

## Command Categories

Use examples instead of treating this section as a complete command inventory:

- Cross-platform examples: `find`, `du`, `grep`, `sed`, `curl`
- Linux-oriented examples: `ps`, `top`, `iostat`, `netstat`, `ifstat`

Linux commands read from `/proc` filesystem.

## Project Status

The command surface is broader than the examples in this file. Treat this document as implementation guidance, not as the canonical command list.

For current command support and planned work:
- See `docs/CMD-DESIGN.md` for supported commands and compatibility scope
- See `PLAN.md` for deferred or planned command work

## Git Commit Guidelines

**Consolidate interim changes**: Use `git rebase -i` or `git commit --amend` to merge non-substantive, consecutive micro-commits.
- Documentation changes (README.md, PLAN.md) should be squashed into the feature commit they belong to
- Avoid fragmenting PRs with "docs: fix typo", "docs: update example" style commits
- Each commit should be atomic and self-contained
