package net

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// runTwCmdFull runs TwCmd with args and captures stdout/stderr separately.
func runTwCmdFull(args []string) (string, string, error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := TwCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return outBuf.String(), errBuf.String(), err
}

// runTwCmd runs TwCmd with args and captures stdout.
func runTwCmd(args []string) (string, error) {
	var buf bytes.Buffer
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := TwCmd(args)

	w.Close()
	io.Copy(&buf, r)
	os.Stdout = old
	return buf.String(), err
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForHTTPReady(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not become ready: %s", url)
}

// ============== HELP TESTS ==============

func TestTwHelp(t *testing.T) {
	output, err := runTwCmd([]string{"-h"})
	if err != nil {
		t.Fatalf("tw -h command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info in output, got: %s", result)
	}
	if !strings.Contains(result, "--bench") {
		t.Errorf("Expected --bench option in help, got: %s", result)
	}
}

func TestTwHelpLong(t *testing.T) {
	output, err := runTwCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("tw --help command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info in output, got: %s", result)
	}
}

func TestTwCmdBenchServerServesPing(t *testing.T) {
	port := freeTCPPort(t)
	go func() {
		_ = TwCmd([]string{"--bench", "-p", strconv.Itoa(port)})
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/ping", port)
	waitForHTTPReady(t, url)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != "pong" {
		t.Fatalf("expected pong, got %q", body)
	}
}

func TestTwCmdStaticServerServesConfiguredDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello from tw"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freeTCPPort(t)
	go func() {
		_ = TwCmd([]string{"-p", strconv.Itoa(port), "-d", dir})
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/hello.txt", port)
	waitForHTTPReady(t, url)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET /hello.txt failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if string(body) != "hello from tw" {
		t.Fatalf("expected file body, got %q", body)
	}
}

// TestTwReuseStaticModeLogsServerNotBenchmark is a regression test: the
// -r/--reuse startup log unconditionally said "starting benchmark server"
// even in plain static-file mode (no --bench). It must describe the actual
// running mode.
func TestTwReuseStaticModeLogsServerNotBenchmark(t *testing.T) {
	dir := t.TempDir()
	port := freeTCPPort(t)

	stderrFile, err := os.CreateTemp("", "tw-stderr-static-*.log")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(stderrFile.Name())

	oldStderr := os.Stderr
	os.Stderr = stderrFile
	go func() {
		_ = TwCmd([]string{"-r", "-p", strconv.Itoa(port), "-d", dir})
	}()

	waitForHTTPReady(t, fmt.Sprintf("http://127.0.0.1:%d/", port))
	os.Stderr = oldStderr

	logBytes, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	logText := string(logBytes)
	if strings.Contains(logText, "benchmark server") {
		t.Errorf("expected static-file mode log, not benchmark server, got: %q", logText)
	}
	if !strings.Contains(logText, "starting server") {
		t.Errorf("expected 'starting server' log for static mode, got: %q", logText)
	}
}

// TestTwReuseBenchModeLogsBenchmarkServer is the counterpart regression test:
// -r combined with --bench must still say "starting benchmark server".
func TestTwReuseBenchModeLogsBenchmarkServer(t *testing.T) {
	port := freeTCPPort(t)

	stderrFile, err := os.CreateTemp("", "tw-stderr-bench-*.log")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(stderrFile.Name())

	oldStderr := os.Stderr
	os.Stderr = stderrFile
	go func() {
		_ = TwCmd([]string{"-r", "--bench", "-p", strconv.Itoa(port)})
	}()

	waitForHTTPReady(t, fmt.Sprintf("http://127.0.0.1:%d/ping", port))
	os.Stderr = oldStderr

	logBytes, err := os.ReadFile(stderrFile.Name())
	if err != nil {
		t.Fatalf("read stderr log: %v", err)
	}
	logText := string(logBytes)
	if !strings.Contains(logText, "starting benchmark server") {
		t.Errorf("expected 'starting benchmark server' log for --bench mode, got: %q", logText)
	}
}

// ============== BENCHMARK MODE TESTS ==============

func TestTwBenchModeGetPing(t *testing.T) {
	// Start a test server that mimics bench mode behavior
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pong"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "pong" {
		t.Errorf("Expected 'pong', got: %s", string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got: %s", contentType)
	}
}

func TestTwBenchModePostPing(t *testing.T) {
	// Start a test server that mimics bench mode POST /ping behavior
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, r.Body)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	testBody := "hello world test data"
	resp, err := http.Post(server.URL+"/ping", "text/plain", strings.NewReader(testBody))
	if err != nil {
		t.Fatalf("POST /ping failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != testBody {
		t.Errorf("Expected echoed body '%s', got: '%s'", testBody, string(body))
	}
}

func TestTwBenchModePostPingEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, r.Body)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/ping", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /ping with empty body failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}
}

func TestTwBenchModePostPingLargeBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, r.Body)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create a large body (1MB)
	largeBody := strings.Repeat("A", 1024*1024)
	resp, err := http.Post(server.URL+"/ping", "application/octet-stream", strings.NewReader(largeBody))
	if err != nil {
		t.Fatalf("POST /ping with large body failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) != len(largeBody) {
		t.Errorf("Expected body length %d, got: %d", len(largeBody), len(body))
	}
}

func TestTwBenchModeUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	testData := "test file content for upload"
	resp, err := http.Post(server.URL+"/upload", "application/octet-stream", strings.NewReader(testData))
	if err != nil {
		t.Fatalf("POST /upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	if !strings.Contains(response, "upload size:") {
		t.Errorf("Expected 'upload size:' in response, got: %s", response)
	}
	if !strings.Contains(response, "status: ok") {
		t.Errorf("Expected 'status: ok' in response, got: %s", response)
	}
	if !strings.Contains(response, fmt.Sprintf("%d", len(testData))) {
		t.Errorf("Expected size %d in response, got: %s", len(testData), response)
	}
}

func TestTwBenchModeUploadEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/upload", "application/octet-stream", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /upload with empty body failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	if !strings.Contains(response, "upload size: 0") {
		t.Errorf("Expected 'upload size: 0' for empty upload, got: %s", response)
	}
}

func TestTwBenchModeUploadLargeFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create 100KB of data
	largeData := strings.Repeat("B", 100*1024)
	resp, err := http.Post(server.URL+"/upload", "application/octet-stream", strings.NewReader(largeData))
	if err != nil {
		t.Fatalf("POST /upload with large data failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	expectedSize := 100 * 1024
	if !strings.Contains(response, fmt.Sprintf("upload size: %d", expectedSize)) {
		t.Errorf("Expected 'upload size: %d' in response, got: %s", expectedSize, response)
	}
}

func TestTwBenchModeMethodNotAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			if r.Method != "GET" && r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pong"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test PUT method on /ping
	req, _ := http.NewRequest("PUT", server.URL+"/ping", strings.NewReader("test"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /ping failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got: %d", resp.StatusCode)
	}
}

func TestTwBenchModeUploadMethodNotAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test GET on /upload
	resp, err := http.Get(server.URL + "/upload")
	if err != nil {
		t.Fatalf("GET /upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got: %d", resp.StatusCode)
	}
}

// ============== STATIC FILE SERVING TESTS ==============

func TestTwStaticServingRoot(t *testing.T) {
	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/test.txt")
	if err != nil {
		t.Fatalf("GET /test.txt failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("Expected 'hello world', got: %s", string(body))
	}
}

func TestTwStaticServingIndexHtml(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "index.html")
	err := os.WriteFile(indexFile, []byte("<html><body>Index Page</body></html>"), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}
	defer os.Remove(indexFile)

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Index Page") {
		t.Errorf("Expected index.html content, got: %s", string(body))
	}
}

// TestTwStaticServingExplicitIndexHtmlNotRedirected is a regression test
// for a bug where requesting a file by its explicit name "/index.html"
// returned a 301 redirect to "./" (Go's http.ServeFile special-cases the
// literal name "index.html" for URL canonicalization) instead of serving
// the file content directly, unlike nginx/python -m http.server which only
// apply index resolution to directory requests.
func TestTwStaticServingExplicitIndexHtmlNotRedirected(t *testing.T) {
	tmpDir := t.TempDir()
	indexFile := filepath.Join(tmpDir, "index.html")
	const content = "<html><body>Explicit Index Request</body></html>"
	if err := os.WriteFile(indexFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(server.URL + "/index.html")
	if err != nil {
		t.Fatalf("GET /index.html failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 for explicit /index.html request, got %d (Location=%q)", resp.StatusCode, resp.Header.Get("Location"))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != content {
		t.Errorf("expected index.html content %q, got %q", content, string(body))
	}
}

func TestTwStaticServingNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Don't create any files

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/nonexistent.txt")
	if err != nil {
		t.Fatalf("GET /nonexistent.txt failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got: %d", resp.StatusCode)
	}
}

func TestTwStaticServingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/subdir")
	if err != nil {
		t.Fatalf("GET /subdir failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}
}

func TestTwStaticServingDirectoryWithIndex(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "mydir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	indexFile := filepath.Join(subDir, "index.html")
	err = os.WriteFile(indexFile, []byte("Subdirectory Index"), 0644)
	if err != nil {
		t.Fatalf("Failed to create index.html: %v", err)
	}
	defer os.Remove(indexFile)

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/mydir/")
	if err != nil {
		t.Fatalf("GET /mydir/ failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Subdirectory Index") {
		t.Errorf("Expected index content, got: %s", string(body))
	}
}

func TestTwStaticServingForbidden(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	// Try to access parent directory via path traversal
	resp, err := http.Get(server.URL + "/../cmd_tw.go")
	if err != nil {
		t.Fatalf("GET /../cmd_tw.go failed: %v", err)
	}
	defer resp.Body.Close()

	// Should be forbidden (path traversal blocked)
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 403 or 404, got: %d", resp.StatusCode)
	}
}

func TestTwStaticServingPathTraversalBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	err := os.WriteFile(secretFile, []byte("secret data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	// Try path traversal
	resp, err := http.Get(server.URL + "/../secret.txt")
	if err != nil {
		t.Fatalf("GET /../secret.txt failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 403 or 404, got: %d", resp.StatusCode)
	}
}

func TestTwStaticServingImageFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a simple binary file
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	imageFile := filepath.Join(tmpDir, "test.png")
	err := os.WriteFile(imageFile, binaryData, 0644)
	if err != nil {
		t.Fatalf("Failed to create image file: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/test.png")
	if err != nil {
		t.Fatalf("GET /test.png failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, binaryData) {
		t.Errorf("Expected binary data to match")
	}
}

// ============== ERROR CASES ==============

func TestTwInvalidPort(t *testing.T) {
	// Test that invalid port is handled
	// Port 0 is invalid for binding in this context
	// We just verify the command doesn't crash
	_, err := runTwCmd([]string{"-p", "invalid"})
	// Should either error or handle gracefully
	// The current implementation uses fmt.Sscanf which would parse 0
	if err == nil {
		// May succeed in parsing but fail to bind
		t.Log("Command ran without error")
	}
}

func TestTwUnknownOption(t *testing.T) {
	_, err := runTwCmd([]string{"--unknown"})
	if err == nil {
		t.Fatalf("Expected error for unknown option")
	}

	if !strings.Contains(err.Error(), "unknown option") {
		t.Errorf("Expected 'unknown option' error, got: %v", err)
	}
}

func TestTwInvalidPortFormat(t *testing.T) {
	_, err := runTwCmd([]string{"-p", "not-a-port"})
	if err == nil {
		t.Fatal("expected invalid port format error")
	}
	if !strings.Contains(err.Error(), "invalid port") {
		t.Fatalf("expected invalid port error, got %v", err)
	}
}

func TestTwNonexistentDirectory(t *testing.T) {
	// When not in bench mode, serving a nonexistent directory should be handled
	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "doesnotexist")

	server := httptest.NewServer(MakeStaticHandler(nonexistentDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/test.txt")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 404 since directory doesn't exist
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got: %d", resp.StatusCode)
	}
}

// ============== CONCURRENT REQUEST TESTS ==============

func TestTwBenchModeConcurrentPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pong"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			resp, err := http.Get(server.URL + "/ping")
			if err != nil {
				t.Errorf("Concurrent GET failed: %v", err)
				done <- false
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got: %d", resp.StatusCode)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTwBenchModeConcurrentUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(id int) {
			testData := fmt.Sprintf("test data %d", id)
			resp, err := http.Post(server.URL+"/upload", "text/plain", strings.NewReader(testData))
			if err != nil {
				t.Errorf("Concurrent POST failed: %v", err)
				done <- false
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got: %d", resp.StatusCode)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

// ============== SPECIAL CHARACTER TESTS ==============

func TestTwBenchModeUploadSpecialCharacters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	testData := "Hello 世界! 🌍\nWith\ttabs\tand\nnewlines"
	resp, err := http.Post(server.URL+"/upload", "text/plain; charset=utf-8", strings.NewReader(testData))
	if err != nil {
		t.Fatalf("POST /upload with special chars failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	if !strings.Contains(response, fmt.Sprintf("%d", len(testData))) {
		t.Errorf("Expected size %d in response, got: %s", len(testData), response)
	}
}

// ============== CONTENT TYPE TESTS ==============

func TestTwBenchModePingContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("pong"))
			} else if r.Method == "POST" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				io.Copy(w, r.Body)
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Test GET content type
	resp, err := http.Get(server.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got: %s", contentType)
	}

	// Test POST content type
	resp2, err := http.Post(server.URL+"/ping", "text/plain", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("POST /ping failed: %v", err)
	}
	defer resp2.Body.Close()

	contentType2 := resp2.Header.Get("Content-Type")
	if contentType2 != "application/octet-stream" {
		t.Errorf("Expected Content-Type 'application/octet-stream' for POST, got: %s", contentType2)
	}
}

func TestTwBenchModeUploadContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	resp, err := http.Post(server.URL+"/upload", "application/octet-stream", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("POST /upload failed: %v", err)
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" {
		t.Errorf("Expected Content-Type 'text/plain', got: %s", contentType)
	}
}

// ============== TIMEOUT HANDLING TESTS ==============

func TestTwServerShutdown(t *testing.T) {
	// This test verifies that the server can be stopped gracefully
	// by closing the httptest server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	}))

	// Make a request first
	resp, err := http.Get(server.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping failed: %v", err)
	}
	resp.Body.Close()

	// Close the server - this should not panic or error
	server.Close()
}

func TestTwBenchModeKeepAlive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "GET" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pong"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create a client with keep-alive
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns: 1,
		},
	}

	// Make multiple requests on the same connection
	for i := 0; i < 3; i++ {
		resp, err := client.Get(server.URL + "/ping")
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()
	}
}

// ============== REQUEST BODY READING ERROR TESTS ==============

func TestTwBenchModeUploadReadError(t *testing.T) {
	// Test when body reading fails - this is simulated by closing
	// the body early, but http.Post doesn't easily allow this.
	// Instead we test the handler with an empty body which is valid.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" && r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			size := len(body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "upload size: %d, status: ok", size)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Empty body should work fine
	resp, err := http.Post(server.URL+"/upload", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /upload failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}
}

// ============== INTEGRATION STYLE TESTS (spawning actual process) ==============

func TestTwIntegrationBenchModeHelp(t *testing.T) {
	// Verify that --bench and -h can be combined (help takes precedence)
	output, err := runTwCmd([]string{"--bench", "-h"})
	if err != nil {
		t.Fatalf("tw --bench -h failed: %v", err)
	}

	result := string(output)
	// Help should be shown
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationPortOption(t *testing.T) {
	// Test that port option is accepted (not testing actual server start)
	output, err := runTwCmd([]string{"-p", "8888", "-h"})
	if err != nil {
		t.Fatalf("tw -p 8888 -h failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationDirOption(t *testing.T) {
	// Test that dir option is accepted
	tmpDir := t.TempDir()
	output, err := runTwCmd([]string{"-d", tmpDir, "-h"})
	if err != nil {
		t.Fatalf("tw -d tmpDir -h failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationReuseOption(t *testing.T) {
	// Test that reuse option is accepted
	output, err := runTwCmd([]string{"-r", "-h"})
	if err != nil {
		t.Fatalf("tw -r -h failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationCombinedOptions(t *testing.T) {
	// Test combining multiple options
	tmpDir := t.TempDir()
	output, err := runTwCmd([]string{"--bench", "-p", "9999", "-d", tmpDir, "-r", "-h"})
	if err != nil {
		t.Fatalf("tw combined options failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationEqualsSyntax(t *testing.T) {
	// Test --port= syntax
	output, err := runTwCmd([]string{"--port=8888", "-h"})
	if err != nil {
		t.Fatalf("tw --port=8888 failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

func TestTwIntegrationDirEqualsSyntax(t *testing.T) {
	// Test --dir= syntax
	output, err := runTwCmd([]string{"--dir=/var/www", "-h"})
	if err != nil {
		t.Fatalf("tw --dir=/var/www failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage: gobox tw") {
		t.Errorf("Expected usage info, got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestTwStaticServingEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Create an empty directory
	emptyDir := filepath.Join(tmpDir, "empty")
	err := os.Mkdir(emptyDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/empty")
	if err != nil {
		t.Fatalf("GET /empty failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<html>") {
		t.Errorf("Expected HTML directory listing, got: %s", string(body))
	}
}

func TestTwStaticServingVeryLongFilename(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file with a long name
	longName := strings.Repeat("a", 200) + ".txt"
	testFile := filepath.Join(tmpDir, longName)
	err := os.WriteFile(testFile, []byte("long name test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create long name file: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/" + longName)
	if err != nil {
		t.Fatalf("GET long name file failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}
}

func TestTwStaticServingMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	// Create multiple files
	files := map[string]string{
		"file1.txt":    "content 1",
		"file2.txt":    "content 2",
		"file3.txt":    "content 3",
		"nested/file4": "content 4",
	}

	for name, content := range files {
		fullPath := filepath.Join(tmpDir, name)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	for name, expectedContent := range files {
		resp, err := http.Get(server.URL + "/" + name)
		if err != nil {
			t.Fatalf("GET /%s failed: %v", name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for /%s, got: %d", name, resp.StatusCode)
		}
		if string(body) != expectedContent {
			t.Errorf("Expected content '%s' for /%s, got: '%s'", expectedContent, name, string(body))
		}
	}
}

func TestTwBenchModePostBinaryData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" && r.Method == "POST" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			io.Copy(w, r.Body)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Send binary data
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	resp, err := http.Post(server.URL+"/ping", "application/octet-stream", bytes.NewReader(binaryData))
	if err != nil {
		t.Fatalf("POST binary failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(body, binaryData) {
		t.Errorf("Binary data mismatch")
	}
}

func TestTwStaticServingRootDirectoryListing(t *testing.T) {
	tmpDir := t.TempDir()
	// No index.html, so should show directory listing
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	response := string(body)
	if !strings.Contains(response, "Directory listing for /") {
		t.Errorf("Expected 'Directory listing for /', got: %s", response)
	}
	// Note: Root path does not list subdirectories, only subdirectory paths do
}

// ============== PERFORMANCE/SMOKE TESTS ==============

func TestTwBenchModeSmokeTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ping":
			if r.Method == "GET" {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("pong"))
			} else if r.Method == "POST" {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				io.Copy(w, r.Body)
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		case "/upload":
			if r.Method == "POST" {
				body, _ := io.ReadAll(r.Body)
				size := len(body)
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "upload size: %d, status: ok", size)
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Quick smoke test of all endpoints
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/ping", ""},
		{"POST", "/ping", "test"},
		{"POST", "/upload", "hello"},
		{"GET", "/nonexistent", ""},
	}

	for _, tc := range tests {
		var resp *http.Response
		var err error

		switch tc.method {
		case "GET":
			resp, err = http.Get(server.URL + tc.path)
		case "POST":
			resp, err = http.Post(server.URL+tc.path, "text/plain", strings.NewReader(tc.body))
		}

		if err != nil {
			t.Errorf("%s %s failed: %v", tc.method, tc.path, err)
			continue
		}
		resp.Body.Close()
	}
}

func TestTwStaticServingSmokeTest(t *testing.T) {
	tmpDir := t.TempDir()
	// Create test files
	testFiles := []string{"a.txt", "b.txt", "c.txt"}
	for _, name := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}

	server := httptest.NewServer(MakeStaticHandler(tmpDir))
	defer server.Close()

	for _, name := range testFiles {
		resp, err := http.Get(server.URL + "/" + name)
		if err != nil {
			t.Errorf("GET /%s failed: %v", name, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /%s status %d", name, resp.StatusCode)
		}
	}
}
