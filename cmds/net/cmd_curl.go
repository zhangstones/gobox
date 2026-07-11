package net

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type resolveHost struct {
	host string
	port string
	addr string
}

type curlCommandError struct {
	err            error
	suppressStderr bool
	exitCode       int
}

type curlFormField struct {
	name     string
	filename string
	data     []byte
}

type curlRequestPayload struct {
	body        []byte
	contentType string
	method      string
}

func (e curlCommandError) Error() string {
	return e.err.Error()
}

func (e curlCommandError) Unwrap() error {
	return e.err
}

func (e curlCommandError) SuppressCLIError() bool {
	return e.suppressStderr
}

func (e curlCommandError) ExitCode() int {
	if e.exitCode != 0 {
		return e.exitCode
	}
	return 2
}

// curlCmd implements curl functionality
func CurlCmd(args []string) error {
	var (
		// Basic options
		silent          bool
		showError       bool
		outputFile      string
		remoteName      bool
		followRedirects bool
		head            bool
		writeOut        string
		maxTime         time.Duration
		request         string
		headers         []string
		postData        string
		uploadFile      string
		formFields      []curlFormField
		insecure        bool
		connectTimeout  time.Duration
		resolveHosts    []resolveHost
		failOnError     bool
		showHeaders     bool

		// Benchmark mode
		benchMode      bool
		concurrent     int
		totalRequests  int
		warmupRequests int
		requestTimeout time.Duration
	)

	// Parse flags manually
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-s" || arg == "--silent":
			silent = true
		case arg == "-S" || arg == "--show-error":
			showError = true
		case arg == "-o" || arg == "--output":
			if i+1 >= len(args) {
				return fmt.Errorf("-o requires an argument")
			}
			i++
			outputFile = args[i]
		case arg == "-O" || arg == "--remote-name":
			remoteName = true
		case arg == "-L" || arg == "--location":
			followRedirects = true
		case arg == "-I" || arg == "--head":
			head = true
		case arg == "-w" || arg == "--write-out":
			if i+1 >= len(args) {
				return fmt.Errorf("-w requires an argument")
			}
			i++
			writeOut = args[i]
		case arg == "-m" || arg == "--max-time":
			if i+1 >= len(args) {
				return fmt.Errorf("-m requires an argument")
			}
			i++
			sec, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return fmt.Errorf("invalid timeout value: %s", args[i])
			}
			maxTime = time.Duration(sec * float64(time.Second))
		case arg == "-X" || arg == "--request":
			if i+1 >= len(args) {
				return fmt.Errorf("-X requires an argument")
			}
			i++
			request = args[i]
		case arg == "-H" || arg == "--header":
			if i+1 >= len(args) {
				return fmt.Errorf("-H requires an argument")
			}
			i++
			headers = append(headers, args[i])
		case arg == "-d" || arg == "--data":
			if i+1 >= len(args) {
				return fmt.Errorf("-d requires an argument")
			}
			i++
			postData = args[i]
		case arg == "-T" || arg == "--upload-file":
			if i+1 >= len(args) {
				return fmt.Errorf("-T requires an argument")
			}
			i++
			uploadFile = args[i]
		case arg == "-F" || arg == "--form":
			if i+1 >= len(args) {
				return fmt.Errorf("-F requires an argument")
			}
			i++
			field, err := parseCurlFormField(args[i])
			if err != nil {
				return err
			}
			formFields = append(formFields, field)
		case arg == "-k" || arg == "--insecure":
			insecure = true
		case arg == "--connect-timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("--connect-timeout requires an argument")
			}
			i++
			sec, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return fmt.Errorf("invalid connect-timeout value: %s", args[i])
			}
			connectTimeout = time.Duration(sec * float64(time.Second))
		case arg == "--resolve":
			if i+1 >= len(args) {
				return fmt.Errorf("--resolve requires an argument")
			}
			i++
			parts := strings.Split(args[i], ":")
			if len(parts) != 3 {
				return fmt.Errorf("--resolve requires HOST:PORT:ADDR format")
			}
			resolveHosts = append(resolveHosts, resolveHost{host: parts[0], port: parts[1], addr: parts[2]})
		case arg == "-f" || arg == "--fail":
			failOnError = true
		case arg == "-i" || arg == "--include":
			showHeaders = true
		case arg == "--bench":
			benchMode = true
		case arg == "-c" || arg == "--concurrent":
			if i+1 >= len(args) {
				return fmt.Errorf("-c requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid concurrent value: %s", args[i])
			}
			concurrent = n
		case arg == "-n" || arg == "--requests":
			if i+1 >= len(args) {
				return fmt.Errorf("-n requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return fmt.Errorf("invalid requests value: %s", args[i])
			}
			totalRequests = n
		case arg == "--warmup":
			if i+1 >= len(args) {
				return fmt.Errorf("--warmup requires an argument")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("invalid warmup value: %s", args[i])
			}
			warmupRequests = n
		case arg == "-t" || arg == "--timeout":
			if i+1 >= len(args) {
				return fmt.Errorf("-t requires an argument")
			}
			i++
			sec, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return fmt.Errorf("invalid timeout value: %s", args[i])
			}
			requestTimeout = time.Duration(sec * float64(time.Second))
		case arg == "-h" || arg == "--help":
			printCurlUsage(os.Stdout)
			return nil
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("unknown option: %s", arg)
		default:
			// Not a flag, stop parsing
			goto doneFlags
		}
		i++
	}

doneFlags:
	remaining := args[i:]
	if len(remaining) < 1 {
		return fmt.Errorf("URL required")
	}
	targetURL := remaining[0]
	if (uploadFile != "" && postData != "") || (uploadFile != "" && len(formFields) > 0) || (postData != "" && len(formFields) > 0) {
		return fmt.Errorf("only one of -d, -T, or -F may be used at a time")
	}

	// Set defaults for benchmark mode
	if benchMode {
		if concurrent == 0 {
			concurrent = 1
		}
		if totalRequests == 0 {
			totalRequests = 100
		}
		if requestTimeout == 0 {
			requestTimeout = 10 * time.Second
		}
	}

	// If remote name is specified, derive output filename from URL
	if remoteName {
		u, err := url.Parse(targetURL)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		path := u.Path
		if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
			outputFile = path[idx+1:]
		}
		if outputFile == "" {
			outputFile = "index.html"
		}
	}

	// Create HTTP client
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
		}).DialContext,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}
	if len(resolveHosts) > 0 {
		resolveMap := make(map[string]string, len(resolveHosts))
		for _, host := range resolveHosts {
			resolveMap[net.JoinHostPort(host.host, host.port)] = net.JoinHostPort(host.addr, host.port)
		}
		dialer := &net.Dialer{Timeout: connectTimeout}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			if mapped, ok := resolveMap[addr]; ok {
				return dialer.DialContext(ctx, network, mapped)
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   maxTime,
	}

	if !followRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	// Benchmark mode
	if benchMode {
		return runBench(client, targetURL, request, headers, postData, head,
			concurrent, totalRequests, warmupRequests, requestTimeout, failOnError, silent)
	}

	// Normal mode
	return runSingle(client, targetURL, request, headers, postData, uploadFile, formFields, head,
		outputFile, writeOut, showHeaders, failOnError, silent, showError)
}

// wrapCurlError wraps an error that CurlCmd has not already printed itself
// (e.g. request construction failures); the top-level CLI dispatcher is the
// sole printer for these, gated by the usual silent/showError rule.
func wrapCurlError(err error, silent, showError bool) error {
	if err == nil {
		return nil
	}
	return curlCommandError{
		err:            err,
		suppressStderr: silent && !showError,
		exitCode:       2,
	}
}

// wrapCurlErrorAlreadyPrinted wraps an error for a diagnostic runSingle has
// already printed directly to os.Stderr (or deliberately suppressed under
// -s/--silent); the top-level CLI dispatcher must never print it again.
func wrapCurlErrorAlreadyPrinted(err error, exitCode int) error {
	if err == nil {
		return nil
	}
	return curlCommandError{
		err:            err,
		suppressStderr: true,
		exitCode:       exitCode,
	}
}

func parseCurlFormField(spec string) (curlFormField, error) {
	name, value, ok := strings.Cut(spec, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
		return curlFormField{}, fmt.Errorf("invalid form field %q", spec)
	}
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "@") {
		filePath := strings.TrimPrefix(value, "@")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return curlFormField{}, fmt.Errorf("cannot read form file %s: %w", filePath, err)
		}
		return curlFormField{
			name:     strings.TrimSpace(name),
			filename: filepath.Base(filePath),
			data:     data,
		}, nil
	}
	return curlFormField{
		name: strings.TrimSpace(name),
		data: []byte(value),
	}, nil
}

func buildCurlPayload(method, postData, uploadFile string, formFields []curlFormField, head bool) (*curlRequestPayload, error) {
	payload := &curlRequestPayload{method: "GET"}
	if method != "" {
		payload.method = method
	} else if head {
		payload.method = "HEAD"
	}
	switch {
	case uploadFile != "":
		data, err := os.ReadFile(uploadFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read upload file %s: %w", uploadFile, err)
		}
		payload.body = data
		payload.contentType = "application/octet-stream"
		if method == "" {
			payload.method = "PUT"
		}
	case len(formFields) > 0:
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		for _, field := range formFields {
			part, err := writer.CreateFormFile(field.name, field.filename)
			if err != nil {
				return nil, fmt.Errorf("create form file %s: %w", field.name, err)
			}
			if _, err := part.Write(field.data); err != nil {
				return nil, fmt.Errorf("write form file %s: %w", field.name, err)
			}
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("finalize multipart body: %w", err)
		}
		payload.body = buf.Bytes()
		payload.contentType = writer.FormDataContentType()
		if method == "" {
			payload.method = "POST"
		}
	case postData != "":
		payload.body = []byte(postData)
		payload.contentType = "application/x-www-form-urlencoded"
		if method == "" {
			payload.method = "POST"
		}
	}
	return payload, nil
}

func applyCurlHeaders(req *http.Request, headers []string, contentType string) {
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
}

func buildCurlRequest(targetURL, method string, headers []string, postData, uploadFile string, formFields []curlFormField, head bool) (*http.Request, error) {
	payload, err := buildCurlPayload(method, postData, uploadFile, formFields, head)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if len(payload.body) > 0 {
		body = bytes.NewReader(payload.body)
	}
	req, err := http.NewRequest(payload.method, targetURL, body)
	if err != nil {
		return nil, err
	}
	applyCurlHeaders(req, headers, payload.contentType)
	return req, nil
}

func printCurlUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox curl [OPTION]... URL")
	fmt.Fprintln(w, "Transfer data from a URL.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -s, --silent          Silent mode")
	fmt.Fprintln(w, "  -S, --show-error      Show error even in silent mode")
	fmt.Fprintln(w, "  -o, --output FILE     Write output to FILE")
	fmt.Fprintln(w, "  -O, --remote-name     Use remote file name for output")
	fmt.Fprintln(w, "  -L, --location        Follow redirects")
	fmt.Fprintln(w, "  -I, --head            Fetch headers only")
	fmt.Fprintln(w, "  -w, --write-out FORMAT Output format (e.g. %{http_code})")
	fmt.Fprintln(w, "  -m, --max-time SEC    Maximum time allowed for transfer")
	fmt.Fprintln(w, "  -X, --request CMD     Specify HTTP method (GET/POST/PUT/DELETE)")
	fmt.Fprintln(w, "  -H, --header LINE     Add header to request")
	fmt.Fprintln(w, "  -d, --data DATA       POST data")
	fmt.Fprintln(w, "  -T, --upload-file FILE Upload FILE with PUT")
	fmt.Fprintln(w, "  -F, --form NAME=FILE  Send multipart form data")
	fmt.Fprintln(w, "  -k, --insecure        Ignore certificate errors")
	fmt.Fprintln(w, "  --connect-timeout SEC Connection timeout")
	fmt.Fprintln(w, "  --resolve HOST:PORT:ADDR Force resolve HOST:PORT to ADDR")
	fmt.Fprintln(w, "  -f, --fail            Fail on HTTP 4xx/5xx errors")
	fmt.Fprintln(w, "  -i, --include         Include response headers in output")
	fmt.Fprintln(w, "  -h, --help            Show this help")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Benchmark mode (--bench):")
	fmt.Fprintln(w, "  -c, --concurrent=N    Number of concurrent requests (default 1)")
	fmt.Fprintln(w, "  -n, --requests=N      Total number of requests (default 100)")
	fmt.Fprintln(w, "  --warmup=N           Number of warmup requests (default 0)")
	fmt.Fprintln(w, "  -t, --timeout=SEC    Request timeout (default 10s)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Write-out formats:")
	fmt.Fprintln(w, "  %{http_code}         HTTP status code")
	fmt.Fprintln(w, "  %{time_total}        Total time in seconds")
	fmt.Fprintln(w, "  %{size_download}     Download size in bytes")
	fmt.Fprintln(w, "  %{url_effective}     Effective URL")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox curl https://example.com")
	fmt.Fprintln(w, "  gobox curl -o output.html https://example.com")
	fmt.Fprintln(w, "  gobox curl -I https://example.com")
	fmt.Fprintln(w, "  gobox curl -w 'Status: %{http_code}' https://example.com")
	fmt.Fprintln(w, "  gobox curl -X POST -d 'data=test' https://example.com")
	fmt.Fprintln(w, "  gobox curl -T file.bin https://example.com/upload")
	fmt.Fprintln(w, "  gobox curl -F file=@archive.tar.gz https://example.com/upload")
	fmt.Fprintln(w, "  gobox curl --bench -c 10 -n 100 https://example.com")
}

func runSingle(client *http.Client, targetURL, method string, headers []string, postData, uploadFile string, formFields []curlFormField,
	head bool, outputFile, writeOut string, showHeaders, failOnError, silent, showError bool) error {

	req, err := buildCurlRequest(targetURL, method, headers, postData, uploadFile, formFields, head)
	if err != nil {
		return wrapCurlError(err, silent, showError)
	}

	start := time.Now()

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		if !silent || showError {
			fmt.Fprintf(os.Stderr, "curl: %v\n", err)
		}
		return wrapCurlErrorAlreadyPrinted(fmt.Errorf("request failed: %w", err), 2)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)

	// Handle fail on error
	if failOnError && resp.StatusCode >= 400 {
		if !silent || showError {
			fmt.Fprintf(os.Stderr, "curl: HTTP error %d\n", resp.StatusCode)
		}
		return wrapCurlErrorAlreadyPrinted(fmt.Errorf("HTTP error %d", resp.StatusCode), 22)
	}

	// Build output
	var output io.Writer

	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("cannot create output file: %w", err)
		}
		output = f
		defer f.Close()
	} else {
		output = os.Stdout
	}

	// Write headers if requested (or if this is a HEAD request)
	if showHeaders || head {
		fmt.Fprintf(output, "HTTP/%.1f %s\n", float64(resp.ProtoMajor)+float64(resp.ProtoMinor)/10, resp.Status)
		for k, v := range resp.Header {
			fmt.Fprintf(output, "%s: %s\n", k, v[0])
		}
		if !head {
			fmt.Fprintln(output, "")
		}
	}

	// Write body if not head
	if !head {
		_, err = io.Copy(output, resp.Body)
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
	}

	// Write format output
	if writeOut != "" {
		fmt.Fprintf(os.Stdout, "%s", formatWriteOut(writeOut, resp, elapsed))
	}

	return nil
}

func formatWriteOut(format string, resp *http.Response, elapsed time.Duration) string {
	result := format

	// %{http_code}
	result = strings.ReplaceAll(result, "%{http_code}", strconv.Itoa(resp.StatusCode))

	// %{time_total}
	result = strings.ReplaceAll(result, "%{time_total}", strconv.FormatFloat(elapsed.Seconds(), 'f', 3, 64))

	// %{size_download}
	result = strings.ReplaceAll(result, "%{size_download}", strconv.FormatInt(resp.ContentLength, 10))

	// %{url_effective}
	result = strings.ReplaceAll(result, "%{url_effective}", resp.Request.URL.String())

	return result
}

// Benchmark mode
type benchResult struct {
	latency    time.Duration
	err        error
	statusCode int
}

func runBench(client *http.Client, targetURL, method string, headers []string, postData string,
	head bool, concurrent, totalRequests, warmupRequests int, requestTimeout time.Duration,
	failOnError, silent bool) error {

	// Warmup
	if !silent && warmupRequests > 0 {
		fmt.Fprintf(os.Stderr, "Warming up %d requests...\n", warmupRequests)
	}
	for i := 0; i < warmupRequests; i++ {
		_, _ = doRequest(client, targetURL, method, headers, postData, head, requestTimeout, failOnError)
	}

	// Prepare work channel
	benchStart := time.Now()
	workCh := make(chan int, concurrent)
	results := make(chan benchResult, totalRequests)

	// WaitGroup for concurrent workers
	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range workCh {
				start := time.Now()
				statusCode, err := doRequest(client, targetURL, method, headers, postData, head, requestTimeout, failOnError)
				latency := time.Since(start)
				results <- benchResult{latency: latency, err: err, statusCode: statusCode}
			}
		}()
	}

	// Send work
	go func() {
		for i := 0; i < totalRequests; i++ {
			workCh <- i
		}
		close(workCh)
	}()

	// Wait for all workers
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var latencies []time.Duration
	var failed int

	for r := range results {
		if r.err != nil {
			failed++
		} else {
			latencies = append(latencies, r.latency)
		}
	}

	// Calculate statistics
	if len(latencies) == 0 {
		fmt.Fprintf(os.Stdout, "Requests: %d, Concurrency: %d, Failed: %d\n", totalRequests, concurrent, failed)
		fmt.Fprintf(os.Stdout, "Latency: no successful requests\n")
		return nil
	}

	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	min := latencies[0]
	max := latencies[len(latencies)-1]
	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	mean := sum / time.Duration(len(latencies))

	p50 := latencies[len(latencies)*50/100]
	p90 := latencies[len(latencies)*90/100]
	p99 := latencies[len(latencies)*99/100]

	totalTime := time.Since(benchStart).Seconds()
	if totalTime < 0.001 {
		totalTime = 0.001
	}
	throughput := float64(len(latencies)) / totalTime

	fmt.Fprintf(os.Stdout, "Requests: %d, Concurrency: %d, Failed: %d\n", totalRequests, concurrent, failed)
	fmt.Fprintf(os.Stdout, "Latency: min=%.0fms, max=%.0fms, mean=%.0fms, p50=%.0fms, p90=%.0fms, p99=%.0fms\n",
		min.Seconds()*1000, max.Seconds()*1000, mean.Seconds()*1000,
		p50.Seconds()*1000, p90.Seconds()*1000, p99.Seconds()*1000)
	fmt.Fprintf(os.Stdout, "Throughput: %.0f req/s, Total time: %.1fs\n",
		throughput, totalTime)

	return nil
}

func doRequest(client *http.Client, targetURL, method string, headers []string, postData string,
	head bool, timeout time.Duration, failOnError bool) (int, error) {
	req, err := buildCurlRequest(targetURL, method, headers, postData, "", nil, head)
	if err != nil {
		return 0, err
	}
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(req.Context(), timeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	io.Copy(io.Discard, resp.Body)

	if failOnError && resp.StatusCode >= 400 {
		return resp.StatusCode, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	return resp.StatusCode, nil
}
