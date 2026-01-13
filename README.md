# gobox

A lightweight, BusyBox-like utility toolset written in Go. This project implements a minimal set of Unix/Linux command-line tools in pure Go, designed to be a portable alternative to BusyBox for system administration and file management tasks.

**Version:** 0.1 (partial implementation)

## Project Overview

gobox is an AI-assisted implementation of essential Unix/Linux utilities. It provides a single binary with multiple command modes, similar to BusyBox, allowing for resource-efficient deployment and portability across different platforms.

## Features & Supported Commands

### 1. **find** - File Search Utility
Search for files in a directory hierarchy with flexible filtering options.

**Options:**
- `-name <pattern>` - Match files by name using shell glob patterns (*, ?, [abc])
- `-type <type>` - Filter by type: `f` (file) or `d` (directory)
- `-maxdepth <depth>` - Limit search depth
- `-mindepth <depth>` - Set minimum search depth
- `-empty` - Match empty files or directories
- `-print` - Print matched paths (enabled by default)

**Usage:**
```bash
gobox find /path -name "*.txt" -type f
gobox find . -type d -empty -maxdepth 2
```

### 2. **du** - Disk Usage Report
Summarize disk usage of files and directories.

**Options:**
- `-h` - Display sizes in human-readable format (B, KB, MB, GB, etc.)
- `-s` - Show only summary (total size of each argument)

**Usage:**
```bash
gobox du -h /var/log
gobox du -s -h .
```

**Features:**
- Recursive directory traversal
- Handles permission errors gracefully
- Human-readable byte size formatting

### 3. **ps** - Process List
List running processes with detailed information (Linux-focused).

**Options:**
- `-e` - Show all processes
- `-f` - Full format (display PPID and executable path)
- `-sort <key>` - Sort by: `pid` (default), `cpu`, `rss`, `vms`, `cmd`
- `-r` - Reverse sort order
- `-name <filter>` - Filter by substring in command name/cmdline
- `-n <count>` - Show only N entries (0 = all)
- `-i <milliseconds>` - CPU sampling interval (default: 500ms)
- `-l <length>` - Max command length (0 = unlimited, default: 40 chars)

**Usage:**
```bash
gobox ps -e -f -sort cpu -r
gobox ps -name "java" -sort rss
gobox ps -n 10
```

**Features:**
- Linux-specific: Reads from `/proc` for detailed CPU and memory stats
- CPU percentage calculation via sampling
- Shows PPID, executable, cmdline, VMS, RSS memory
- Smart truncation of long command names when output is to terminal

### 4. **top** - Real-time Process Viewer
Display a dynamic, real-time view of running processes.

**Options:**
- `-d <seconds>` - Delay between updates (default: 2 seconds)
- `-n <iterations>` - Number of iterations (0 = infinite, default: 5)
- `-sort <key>` - Sort by: `pid`, `cpu`, `rss`, `vms`, `cmd`
- `-r` - Reverse sort order (enabled by default)

**Usage:**
```bash
gobox top -d 1 -n 10
gobox top -sort cpu
```

**Features:**
- Clears screen between updates for live view
- Reuses ps command logic for process gathering
- Configurable refresh interval and iteration count

### 5. **iostat** - Block Device I/O Statistics
Print block device IOPS and throughput based on cgroup metrics (Linux only).

**Options:**
- `-i <seconds>` - Sample interval (default: 1 second)
- `-n <count>` - Number of samples to take (default: 1)
- `-H` - Humanize IOPS and throughput (default: true, shows K/M notation)
- `-z` - Show only devices with non-zero I/O rates

**Usage:**
```bash
gobox iostat -i 2 -n 5
gobox iostat -z
```

**Features:**
- Supports Linux cgroup v2 (`io.stat`) and cgroup v1 (`blkio.*`) metrics
- Reads bytes and I/O operations (IOPS)
- Maps device major:minor IDs to device names via `/sys/dev/block`
- Handles permission errors gracefully
- Linux-only: Returns error on non-Linux systems

### 6. **netstat** - Network Connection Statistics
Display network device and connection statistics from Linux `/proc/net`.

**Options:**
- `-state <states>` - Filter by connection state (comma-separated, e.g., `LISTEN,ESTABLISHED`)
- `-port <port>` - Filter by local or remote port number
- `-sort <key>` - Sort by: `recvq`, `sendq`, `local`, `remote`, `pid`

**Usage:**
```bash
gobox netstat -state LISTEN
gobox netstat -port 8080
gobox netstat -sort recvq
```

**Features:**
- Parses `/proc/net/tcp`, `/proc/net/tcp6`, `/proc/net/udp`, `/proc/net/udp6`
- Shows receive/send queue sizes
- Maps inodes to process IDs and names
- IPv6 support
- Linux-only implementation

### 7. **xargs** - Build and Execute Commands from Input
Build and execute command lines from standard input.

**Options:**
- `-i, -I <placeholder>` - Replace mode with custom placeholder (default: `{}`)
- `-d <delimiter>` - Input delimiter (default: newline)
- `-n <count>` - Maximum arguments per command invocation
- `-P <processes>` - Maximum parallel processes (default: 1)
- `-v` - Verbose: print commands before executing
- `-r` - No-run: don't execute command if no input provided

**Usage:**
```bash
find . -name "*.txt" | gobox xargs -P 4 grep "pattern"
echo -e "file1\nfile2\nfile3" | gobox xargs -i rm {}
cat list.txt | gobox xargs -n 5 process_batch
```

**Features:**
- Replace mode for flexible command building
- Append mode for batch processing
- Parallel execution support with semaphore control
- Custom input delimiters
- Verbose output for debugging
- Graceful stdin/stdout/stderr handling

## Global Options

All commands support:
- `-h, --help` - Display help for the command
- `--version, -v, version` - Show gobox version

**Global usage:**
```bash
gobox --help
gobox --version
```

## Requirements & Dependencies

- **Go:** 1.20 or later
- **Platform Support:**
  - Cross-platform: `find`, `du`, `xargs`
  - Linux-specific: `ps`, `top`, `iostat`, `netstat`
- **External Dependencies:**
  - `github.com/mitchellh/go-ps` - Process listing (for cross-platform ps fallback)

## Implementation Notes

### Architecture
- Single-binary design with command dispatching via string matching
- Each command implemented in its own file: `cmd_<command>.go`
- Shared utilities in `utils.go` (e.g., `isStdoutTerminal()`, `humanSize()`)
- Main dispatcher in `main.go`

### Limitations & Design Choices
1. **Partial BusyBox Implementation:** Not all flags from original BusyBox are supported; focuses on commonly-used options
2. **Linux-First:** Some commands (ps, iostat, netstat) are optimized for Linux `/proc` filesystem
3. **Graceful Degradation:** Commands continue on permission errors when traversing directories
4. **No External Commands:** Pure Go implementation with minimal dependencies
5. **Terminal Detection:** Smart output formatting that detects TTY for intelligent command truncation

### Error Handling
- Exit code 1: No command provided or help displayed
- Exit code 2: Command execution error
- Exit code 127: Unknown command
- Commands return descriptive error messages to stderr

## Usage Examples

### Find all Python files over 1MB
```bash
gobox find /project -name "*.py" -type f
```

### Show disk usage of directories
```bash
gobox du -h -s /home /var /tmp
```

### Monitor processes by CPU usage
```bash
gobox top -d 1 -sort cpu
```

### Filter network connections
```bash
gobox netstat -state LISTEN -port 8080
```

### Batch process files in parallel
```bash
ls *.log | gobox xargs -P 4 -i gzip {}
```

### Monitor disk I/O
```bash
gobox iostat -i 1 -n 10 -z
```

## Future Enhancements

- Additional commands (grep, sed, awk, ls, etc.)
- More flag compatibility with standard Unix tools
- Cross-platform support for Windows-specific variants of commands
- Performance optimizations for large datasets
- Configuration file support
- Man page documentation
