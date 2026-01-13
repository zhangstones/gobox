package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// xargsCmd implements a basic subset of xargs
func xargsCmd(args []string) error {
	xargsFlags := flag.NewFlagSet("xargs", flag.ContinueOnError)
	xargsFlags.SetOutput(os.Stderr)
	replaceStr := xargsFlags.String("i", "", "replace string (same as -I, use {} as default)")
	replaceStr2 := xargsFlags.String("I", "", "replace string with custom placeholder")
	delimiter := xargsFlags.String("d", "\n", "input delimiter (default: newline)")
	numArgs := xargsFlags.Int("n", 0, "max number of arguments per command invocation")
	maxProcs := xargsFlags.Int("P", 1, "max number of parallel processes")
	verbose := xargsFlags.Bool("v", false, "print commands before executing")
	noRun := xargsFlags.Bool("r", false, "do not run command if no input")

	xargsFlags.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: gobox xargs [OPTIONS] [COMMAND [ARGS...]]")
		fmt.Fprintln(os.Stderr, "Build and execute command lines from standard input.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Options:")
		xargsFlags.PrintDefaults()
	}

	if err := xargsFlags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Get command and its arguments
	cmdArgs := xargsFlags.Args()
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"echo"}
	}

	// Determine replace string
	replaceString := ""
	hasReplace := false
	if *replaceStr != "" {
		replaceString = *replaceStr
		hasReplace = true
	} else if *replaceStr2 != "" {
		replaceString = *replaceStr2
		hasReplace = true
	}

	// If -i or -I flag was specified, use default {} if no value provided
	if hasReplace && replaceString == "" {
		replaceString = "{}"
	}

	// Read input
	var inputs []string
	scanner := bufio.NewScanner(os.Stdin)
	if *delimiter != "\n" {
		scanner = bufio.NewScanner(os.Stdin)
		scanner.Split(makeDelimiterSplitFunc(*delimiter))
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			inputs = append(inputs, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// If no input and -r flag is set, don't run command
	if len(inputs) == 0 && *noRun {
		return nil
	}

	// If no input, run command once
	if len(inputs) == 0 {
		if *verbose {
			fmt.Fprintf(os.Stderr, "%s\n", strings.Join(cmdArgs, " "))
		}
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Process inputs in batches and execute commands in parallel
	if replaceString != "" {
		// Replace mode: replace placeholder with each input
		return executeReplaceMode(cmdArgs, inputs, replaceString, *verbose, *maxProcs)
	} else {
		// Append mode: append inputs to command
		return executeAppendMode(cmdArgs, inputs, *numArgs, *verbose, *maxProcs)
	}
}

// executeReplaceMode replaces the placeholder with inputs
func executeReplaceMode(baseCmd []string, inputs []string, replaceString string, verbose bool, maxProcs int) error {
	semaphore := make(chan struct{}, maxProcs)
	ready := make(chan struct{})
	var wg sync.WaitGroup
	var lastErr error
	var mu sync.Mutex

	for _, input := range inputs {
		semaphore <- struct{}{} // Acquire semaphore before launching goroutine
		wg.Add(1)
		go func(inp string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore after completion

			// Signal that this goroutine has acquired the semaphore
			ready <- struct{}{}

			// Build command with replacement
			cmdArgs := make([]string, len(baseCmd))
			copy(cmdArgs, baseCmd)

			for i, arg := range cmdArgs {
				cmdArgs[i] = strings.ReplaceAll(arg, replaceString, inp)
			}

			if verbose {
				fmt.Fprintf(os.Stderr, "%s\n", strings.Join(cmdArgs, " "))
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			}
		}(input)
		<-ready // Wait for goroutine to acquire semaphore before launching next one
	}

	wg.Wait()
	return lastErr
}

// executeAppendMode appends inputs to the command in batches
func executeAppendMode(baseCmd []string, inputs []string, batchSize int, verbose bool, maxProcs int) error {
	if batchSize <= 0 {
		batchSize = len(inputs)
	}

	semaphore := make(chan struct{}, maxProcs)
	ready := make(chan struct{})
	var wg sync.WaitGroup
	var lastErr error
	var mu sync.Mutex

	for i := 0; i < len(inputs); i += batchSize {
		end := i + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		batch := inputs[i:end]
		semaphore <- struct{}{} // Acquire semaphore before launching goroutine
		wg.Add(1)

		go func(batchItems []string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore after completion

			// Signal that this goroutine has acquired the semaphore
			ready <- struct{}{}

			// Build command with batch items
			cmdArgs := make([]string, len(baseCmd))
			copy(cmdArgs, baseCmd)
			cmdArgs = append(cmdArgs, batchItems...)

			if verbose {
				fmt.Fprintf(os.Stderr, "%s\n", strings.Join(cmdArgs, " "))
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
			}
		}(batch)
		<-ready // Wait for goroutine to acquire semaphore before launching next one
	}

	wg.Wait()
	return lastErr
}

// makeDelimiterSplitFunc creates a split function for custom delimiters
func makeDelimiterSplitFunc(delimiter string) bufio.SplitFunc {
	delim := []byte(delimiter)
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) == 0 {
			if atEOF {
				return 0, nil, nil
			}
			return 0, nil, nil
		}

		// Find delimiter
		idx := strings.Index(string(data), delimiter)
		if idx >= 0 {
			return idx + len(delim), data[:idx], nil
		}

		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	}
}
