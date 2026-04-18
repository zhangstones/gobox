package text

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// TailCmdWithContext implements the tail command with context support
func TailCmdWithContext(ctx context.Context, args []string) error {
	var (
		lines         = 10    // default number of lines
		follow        = false // -f: follow mode
		followByName  = false // --follow=name: follow by filename
		retry         = false // --retry: keep trying to open file
		quiet         = false // -q: quiet mode
		sleepInterval = 1.0   // -s: sleep interval in seconds
		pid           = -1    // --pid: stop following when process dies
		showHelp      = false
	)

	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-n" || arg == "--lines":
			if i+1 >= len(args) {
				return fmt.Errorf("-n/--lines requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", args[i])
			}
			lines = n
		case strings.HasPrefix(arg, "-n="):
			n, err := strconv.Atoi(arg[3:])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", arg[3:])
			}
			lines = n
		case arg == "--lines=":
			if i+1 >= len(args) {
				return fmt.Errorf("--lines= requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid number of lines: %s", args[i])
			}
			lines = n
		case arg == "-f" || arg == "--follow":
			follow = true
		case arg == "--follow=name":
			follow = true
			followByName = true
		case arg == "--retry":
			retry = true
		case arg == "-q" || arg == "--quiet" || arg == "--silent":
			quiet = true
		case arg == "-s" || arg == "--sleep-interval":
			if i+1 >= len(args) {
				return fmt.Errorf("-s/--sleep-interval requires an argument")
			}
			i++
			f, err := strconv.ParseFloat(args[i], 64)
			if err != nil || f <= 0 {
				return fmt.Errorf("invalid sleep interval: %s", args[i])
			}
			sleepInterval = f
		case strings.HasPrefix(arg, "-s="):
			f, err := strconv.ParseFloat(arg[3:], 64)
			if err != nil || f <= 0 {
				return fmt.Errorf("invalid sleep interval: %s", arg[3:])
			}
			sleepInterval = f
		case arg == "--sleep-interval=":
			if i+1 >= len(args) {
				return fmt.Errorf("--sleep-interval= requires an argument")
			}
			i++
			f, err := strconv.ParseFloat(args[i], 64)
			if err != nil || f <= 0 {
				return fmt.Errorf("invalid sleep interval: %s", args[i])
			}
			sleepInterval = f
		case strings.HasPrefix(arg, "--pid="):
			p, err := strconv.Atoi(arg[6:])
			if err != nil || p <= 0 {
				return fmt.Errorf("invalid PID: %s", arg[6:])
			}
			pid = p
		case arg == "-h" || arg == "--help":
			showHelp = true
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}

doneFlags:
	if showHelp {
		printTailUsage(os.Stdout)
		return nil
	}

	files := args[i:]
	multipleFiles := len(files) > 1

	// If no files, read from stdin (but not in follow mode)
	if len(files) == 0 {
		if follow {
			return fmt.Errorf("cannot follow stdin in follow mode")
		}
		if err := tailReader(os.Stdin, os.Stdout, lines); err != nil {
			return err
		}
		return nil
	}

	// Follow mode
	if follow {
		return tailFollow(ctx, files, lines, followByName, retry, quiet, multipleFiles, sleepInterval, pid)
	}

	// Normal mode - process files
	for _, file := range files {
		if multipleFiles && !quiet {
			fmt.Printf("==> %s <==\n", file)
		}
		if err := tailFile(file, os.Stdout, lines); err != nil {
			return err
		}
		if multipleFiles && !quiet && file != files[len(files)-1] {
			fmt.Println()
		}
	}

	return nil
}

// tailCmd implements the tail command
func TailCmd(args []string) error {
	return TailCmdWithContext(context.Background(), args)
}

func printTailUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox tail [OPTIONS] [FILE...]")
	fmt.Fprintln(w, "Print the last lines of a file.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -n NUM, --lines=NUM     Print the last NUM lines (default 10)")
	fmt.Fprintln(w, "  -f, --follow            Output appended data as the file grows")
	fmt.Fprintln(w, "  --follow=name           Follow by filename (handle log rotation)")
	fmt.Fprintln(w, "  --retry                 Keep trying to open file if not present")
	fmt.Fprintln(w, "  -q, --quiet             Never print headers giving file names")
	fmt.Fprintln(w, "  -s SEC, --sleep-interval=SEC  Seconds between iterations (default 1)")
	fmt.Fprintln(w, "  --pid=PID               Stop when process PID exits")
	fmt.Fprintln(w, "  -h, --help              Show this help message")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox tail file.txt              Print last 10 lines")
	fmt.Fprintln(w, "  gobox tail -n 20 file.txt        Print last 20 lines")
	fmt.Fprintln(w, "  gobox tail -f file.txt            Follow file growth")
	fmt.Fprintln(w, "  gobox tail --follow=name file.txt  Follow with log rotation")
	fmt.Fprintln(w, "  gobox tail --pid=123 -f file.txt  Stop when PID 123 exits")
}

func tailReader(r io.Reader, w io.Writer, n int) error {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return nil
}

func tailFile(filename string, w io.Writer, n int) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", filename, err)
	}
	defer file.Close()

	return tailReader(file, w, n)
}

// tailFileInfo tracks file info for follow mode
type tailFileInfo struct {
	name      string
	ino       uint64
	dev       uint64
	offset    int64
	reader    *os.File
	following bool
}

// getFileInode tries to get the inode number of a file
func getFileInode(f *os.File) (uint64, uint64, error) {
	stat, err := f.Stat()
	if err != nil {
		return 0, 0, err
	}
	// Use sys() to get underlying stat info
	if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
		return sys.Ino, sys.Dev, nil
	}
	return 0, 0, fmt.Errorf("cannot get inode info")
}

// tailFollow implements the -f (follow) functionality
func tailFollow(ctx context.Context, files []string, lines int, followByName, retry, quiet, multipleFiles bool, sleepInterval float64, pid int) error {
	// Set up signal handling for SIGINT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	trackedFiles := make(map[string]*tailFileInfo)

	finishChan := make(chan struct{})
	var procFinished bool

	// Check if pid exited
	if pid > 0 {
		go func() {
			for {
				proc, err := os.FindProcess(pid)
				if err != nil {
					break
				}
				if proc.Signal(syscall.Signal(0)) != nil {
					// Process no longer exists
					select {
					case finishChan <- struct{}{}:
					default:
					}
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}()
	}

	// Open initial files
	for _, filename := range files {
		f, err := os.Open(filename)
		if err != nil {
			if retry {
				// Will retry later
				trackedFiles[filename] = &tailFileInfo{name: filename, offset: 0, following: true}
				continue
			}
			return fmt.Errorf("cannot open %s: %w", filename, err)
		}
		stat, _ := f.Stat()
		ino, dev, _ := getFileInode(f)
		trackedFiles[filename] = &tailFileInfo{
			name:      filename,
			ino:       ino,
			dev:       dev,
			offset:    stat.Size(),
			reader:    f,
			following: true,
		}
		// Print last n lines of initial content
		if !quiet && multipleFiles {
			fmt.Printf("==> %s <==\n", filename)
		}
		if err := tailFileReader(f, os.Stdout, lines); err != nil {
			f.Close()
			return err
		}
	}

	interval := time.Duration(float64(time.Second) * sleepInterval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - clean up and exit
			for _, fi := range trackedFiles {
				if fi.reader != nil {
					fi.reader.Close()
				}
			}
			return nil
		case <-sigChan:
			// Clean up and exit
			for _, fi := range trackedFiles {
				if fi.reader != nil {
					fi.reader.Close()
				}
			}
			return nil
		case <-finishChan:
			procFinished = true
		case <-ticker.C:
			if procFinished {
				// Process died, exit gracefully
				for _, fi := range trackedFiles {
					if fi.reader != nil {
						fi.reader.Close()
					}
				}
				return nil
			}

			// Check each file for new content
			for _, fi := range trackedFiles {
				if fi.reader == nil && !retry {
					continue
				}
				if fi.reader == nil && retry {
					// Try to open the file
					f, err := os.Open(fi.name)
					if err != nil {
						continue
					}
					fi.reader = f
					stat, _ := f.Stat()
					ino, dev, _ := getFileInode(f)
					fi.ino = ino
					fi.dev = dev
					fi.offset = stat.Size()
					if !quiet && multipleFiles {
						fmt.Printf("==> %s <==\n", fi.name)
					}
				}

				if followByName {
					if err := tailFollowByName(fi, os.Stdout, lines, quiet, multipleFiles); err != nil {
						return err
					}
				} else {
					if err := tailFollowByFd(fi, os.Stdout, lines); err != nil {
						return err
					}
				}
			}
		}
	}
}

func tailFileReader(f *os.File, w io.Writer, n int) error {
	// Seek to end and read last n lines
	stat, err := f.Stat()
	if err != nil {
		return err
	}

	if stat.Size() == 0 {
		return nil
	}

	// Read from the end
	fileSize := stat.Size()
	var startPos int64 = 0
	if fileSize > 4096 {
		startPos = fileSize - 4096
	}

	_, err = f.Seek(startPos, io.SeekStart)
	if err != nil {
		return err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	// Scanner may not see last line if no newline at end
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	for _, line := range lines {
		fmt.Fprintln(w, line)
	}
	return nil
}

func tailFollowByFd(fi *tailFileInfo, w io.Writer, lines int) error {
	if fi.reader == nil {
		return nil
	}

	stat, err := fi.reader.Stat()
	if err != nil {
		return err
	}

	currentSize := stat.Size()
	if currentSize > fi.offset {
		// New content available
		_, err := fi.reader.Seek(fi.offset, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, fi.reader)
		if err != nil {
			return err
		}
		fi.offset = currentSize
	} else if currentSize < fi.offset {
		// File was truncated, seek to beginning
		fi.offset = 0
		_, err = fi.reader.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}
		io.Copy(w, fi.reader)
		fi.offset = currentSize
	}
	return nil
}

func tailFollowByName(fi *tailFileInfo, w io.Writer, lines int, quiet, multipleFiles bool) error {
	// Reopen file to check for rotation
	f, err := os.Open(fi.name)
	if err != nil {
		return nil // File might not exist yet
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	ino, dev, err := getFileInode(f)
	if err != nil {
		return err
	}

	// Check if file was rotated (new inode)
	if ino != fi.ino || dev != fi.dev {
		// File was rotated, print separator and read last n lines from new file
		if !quiet && multipleFiles {
			fmt.Printf("==> %s (rotation) <==\n", fi.name)
		}
		tailFileReader(f, w, lines)
		fi.ino = ino
		fi.dev = dev
		fi.offset = stat.Size()
		fi.reader.Close()
		fi.reader = f
		return nil
	}

	// No rotation, just check for appended content
	currentSize := stat.Size()
	if currentSize > fi.offset {
		// New content available
		_, err := f.Seek(fi.offset, io.SeekStart)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		if err != nil {
			return err
		}
		fi.offset = currentSize
	} else if currentSize < fi.offset {
		// File was truncated
		if !quiet && multipleFiles {
			fmt.Printf("==> %s (truncated) <==\n", fi.name)
		}
		tailFileReader(f, w, lines)
		fi.offset = stat.Size()
	}

	// Update reader reference
	if fi.reader != f {
		fi.reader.Close()
		fi.reader = f
	}
	return nil
}
