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

gobox is a single-binary utility toolkit inspired by BusyBox. A single executable dispatches to different commands based on argv[0] or first argument.

### Command Implementation Pattern

Each command lives in its own file: `cmd_<name>.go`
- `cmd_find.go`, `cmd_du.go`, `cmd_ps.go`, etc.
- Each file exports a function like `func <name>Cmd(args []string) error`
- main.go switches on the command name and calls the appropriate function

### Command Signature Convention

All command functions follow this pattern:
```go
func <name>Cmd(args []string) error
```

Return `nil` on success, error on failure. Exit codes are handled by main.go:
- Exit 1: help/error
- Exit 2: command execution error
- Exit 127: unknown command

### Testing Requirements

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
- Cross-platform: `find`, `du`, `xargs`, `grep`, `sed`, `head`, `tail`, `sort`, `uniq`, `wc`
- Network: `dig`, `curl`, `nc`, `tw`, `ifstat`, `np`
- System: `ps`, `top`, `iostat`, `netstat`, `md5sum`, `ioperf`, `rand`, `seq`

Run smoke tests before submitting PRs:
```bash
go test ./tests/smoke -v
```

### Documentation Requirements

When adding new features, changing behavior, or fixing bugs that affect user-visible functionality:
1. Update README.md with new command documentation and usage examples
2. Update the command's help text (typically in the command file)
3. Ensure help text and README are consistent with each other

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

- `utils.go` - Common helpers: `isStdoutTerminal()`, `humanSize()`, etc.
- Tests live alongside implementation: `cmd_foo.go` → `cmd_foo_test.go`

### Dependencies

- `github.com/mitchellh/go-ps` - Cross-platform process listing

## Command Categories

**Cross-platform**: `find`, `du`, `xargs`, `grep`, `sed`
**Linux-specific**: `ps`, `top`, `iostat`, `netstat`

Linux commands read from `/proc` filesystem.

## Project Status

Current commands: `find`, `du`, `ps`, `top`, `iostat`, `netstat`, `xargs`, `grep`, `sed`, `head`, `tail`, `curl`, `sort`, `uniq`, `wc`, `nslookup`, `dig`, `nc`, `tw`

Planned additions: `ifstat`, `np` (netping). See PLAN.md for details.

## Git Commit Guidelines

**Consolidate interim changes**: Use `git rebase -i` or `git commit --amend` to merge non-substantive, consecutive micro-commits.
- Documentation changes (README.md, PLAN.md) should be squashed into the feature commit they belong to
- Avoid fragmenting PRs with "docs: fix typo", "docs: update example" style commits
- Each commit should be atomic and self-contained
