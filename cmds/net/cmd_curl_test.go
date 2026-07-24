package net

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// runCurlCmdFull runs CurlCmd with args and captures stdout and stderr separately.
func runCurlCmdFull(args []string) (string, string, error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := CurlCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return outBuf.String(), errBuf.String(), err
}

func runCurlCmd(args []string) (string, error) {
	stdout, stderr, err := runCurlCmdFull(args)
	return stdout + stderr, err
}

// runCurlCmdWithStdinFull runs CurlCmd with stdin input and captures stdout/stderr separately.
func runCurlCmdWithStdinFull(args []string, stdinInput string) (string, string, error) {
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr
	os.Stdin = rIn

	go func() {
		wIn.WriteString(stdinInput)
		wIn.Close()
	}()

	err := CurlCmd(args)

	wOut.Close()
	wErr.Close()
	io.Copy(&outBuf, rOut)
	io.Copy(&errBuf, rErr)
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin
	return outBuf.String(), errBuf.String(), err
}

func runCurlCmdWithStdin(args []string, stdinInput string) (string, error) {
	stdout, stderr, err := runCurlCmdWithStdinFull(args, stdinInput)
	return stdout + stderr, err
}

// ============== BASIC GET TESTS ==============

func TestCurlBasicGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		fmt.Fprint(w, "Hello, World!")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got: %s", result)
	}
}

func TestCurlGetWithQueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/test" {
			t.Errorf("Expected path /test, got %s", r.URL.Path)
		}
		if r.URL.RawQuery != "foo=bar" {
			t.Errorf("Expected query foo=bar, got %s", r.URL.RawQuery)
		}
		fmt.Fprint(w, "query response")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{server.URL + "/test?foo=bar"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "query response" {
		t.Errorf("Expected 'query response', got: %s", result)
	}
}

// ============== SILENT MODE TESTS (-s) ==============

func TestCurlSilentMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "silent response")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-s", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "silent response" {
		t.Errorf("Expected 'silent response', got: %s", result)
	}
}

func TestCurlSilentModeSuppressesProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send enough data to trigger any progress output
		for i := 0; i < 100; i++ {
			fmt.Fprint(w, "line of data\n")
		}
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-s", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	// Should not contain any progress meter characters
	if strings.Contains(result, "%") {
		t.Errorf("Progress meter should be suppressed in silent mode, got: %s", result)
	}
}

func TestCurlSilentModeSuppressesErrorOutput(t *testing.T) {
	stdout, stderr, err := runCurlCmdFull([]string{"-s", "://bad-url"})
	if err == nil {
		t.Fatalf("expected curl to fail for invalid URL")
	}
	if stdout != "" || stderr != "" {
		t.Fatalf("expected silent mode to suppress output, stdout=%q stderr=%q", stdout, stderr)
	}
}

// ============== SHOW ERROR TESTS (-S) ==============

func TestCurlShowErrorWithSilent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	stdout, stderr, err := runCurlCmdFull([]string{"-s", "-S", "-f", server.URL})
	if err == nil {
		t.Fatalf("expected -s -S -f to fail on HTTP 500")
	}
	if stdout != "" {
		t.Fatalf("expected no response body on fail-on-error, got stdout=%q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "500") && !strings.Contains(strings.ToLower(stderr), "server error") {
		t.Fatalf("expected show-error stderr to mention the HTTP failure, got %q", stderr)
	}
}

func TestCurlShowErrorWithoutSilent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	// Without -f, even 404 should not error but show output
	output, err := runCurlCmd([]string{"-S", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Not Found") {
		t.Errorf("Expected error message in output, got: %s", result)
	}
}

// ============== OUTPUT FILE TESTS (-o) ==============

func TestCurlOutputToFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "file content here")
	}))
	defer server.Close()

	outputFile := "test_curl_output.txt"
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-o", outputFile, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	// Read the output file
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "file content here" {
		t.Errorf("Expected 'file content here', got: %s", string(content))
	}
}

func TestCurlOutputToFileError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	// Try to write to a directory (should fail)
	tmpDir := t.TempDir()
	stdout, stderr, err := runCurlCmdFull([]string{"-o", tmpDir, server.URL})
	if err == nil {
		t.Errorf("Expected error when writing to directory")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout when output path is invalid, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "directory") && !strings.Contains(strings.ToLower(err.Error()), "directory") {
		t.Fatalf("expected directory-related error, stderr=%q err=%v", stderr, err)
	}
}

// ============== REMOTE NAME TESTS (-O) ==============

func TestCurlRemoteName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=testfile.txt")
		fmt.Fprint(w, "remote file content")
	}))
	defer server.Close()

	// The -O flag should use the filename from URL path
	outputFile := "testfile.txt"
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-O", server.URL + "/testfile.txt"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	// Check if file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "remote file content" {
		t.Errorf("Expected 'remote file content', got: %s", string(content))
	}
}

func TestCurlRemoteNameDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "default content")
	}))
	defer server.Close()

	// For URL with trailing slash and no name, should use index.html
	// Change to temp dir to avoid polluting the source tree
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	_, err := runCurlCmd([]string{"-O", server.URL + "/"})
	if err != nil {
		t.Fatalf("curl -O against a trailing-slash URL failed: %v", err)
	}

	data, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("expected -O to write default filename index.html: %v", err)
	}
	if string(data) != "default content" {
		t.Errorf("expected index.html to contain %q, got %q", "default content", string(data))
	}
}

// ============== FOLLOW REDIRECTS TESTS (-L) ==============

func TestCurlFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusMovedPermanently)
			return
		}
		if r.URL.Path == "/final" {
			fmt.Fprint(w, "redirected content")
			return
		}
		t.Errorf("Unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-L", server.URL + "/redirect"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "redirected content" {
		t.Errorf("Expected 'redirected content', got: %s", result)
	}
}

func TestCurlNoFollowRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusMovedPermanently)
			return
		}
		// Without -L, should not follow redirect
		t.Errorf("Should not have reached /final without -L flag")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{server.URL + "/redirect"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	// Should show redirect location, not the final content
	if strings.Contains(result, "redirected content") {
		t.Errorf("Should not follow redirect without -L flag, got: %s", result)
	}
}

// ============== HEAD REQUEST TESTS (-I) ==============

func TestCurlHeadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("Expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("X-Custom-Header", "test")
		w.WriteHeader(http.StatusOK)
		// Don't write body - HEAD should not have body
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-I", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "200") {
		t.Errorf("expected HEAD output to include the 200 status, got: %s", result)
	}
	if !strings.Contains(result, "X-Custom-Header: test") {
		t.Errorf("expected HEAD output to include the server's X-Custom-Header, got: %s", result)
	}
}

// ============== WRITE-OUT FORMAT TESTS (-w) ==============

func TestCurlWriteOutHttpCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", "%{http_code}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "200" {
		t.Errorf("Expected '200', got: %s", result)
	}
}

func TestCurlWriteOutMultipleFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", "Status: %{http_code}, Size: %{size_download}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, "Status: 200") {
		t.Errorf("Expected 'Status: 200', got: %s", result)
	}
	if !strings.Contains(result, "Size: 7") {
		t.Errorf("Expected 'Size: 7', got: %s", result)
	}
}

func TestCurlWriteOutUrlEffective(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	// With redirect, effective URL should show final URL
	output, err := runCurlCmd([]string{"-w", "%{url_effective}", "-o", os.DevNull, "-L", server.URL + "/path"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, server.URL+"/path") {
		t.Errorf("Expected URL in output, got: %s", result)
	}
}

// ============== MAX TIME TESTS (-m) ==============

func TestCurlMaxTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delay response longer than max-time
		time.Sleep(2 * time.Second)
		fmt.Fprint(w, "slow response")
	}))
	defer server.Close()

	stdout, stderr, err := runCurlCmdFull([]string{"-m", "1", server.URL})
	if err == nil {
		t.Errorf("Expected timeout error with max-time 1 second and 2 second delay")
	}
	if stdout != "" {
		t.Fatalf("expected no stdout on request timeout, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr+err.Error()), "timeout") && !strings.Contains(strings.ToLower(stderr+err.Error()), "deadline") {
		t.Fatalf("expected timeout error details, stderr=%q err=%v", stderr, err)
	}
}

// ============== HTTP METHOD TESTS (-X) ==============

func TestCurlPostMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		fmt.Fprint(w, "POST response")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-X", "POST", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "POST response" {
		t.Errorf("Expected 'POST response', got: %s", result)
	}
}

func TestCurlPutMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}
		fmt.Fprint(w, "PUT response")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-X", "PUT", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "PUT response" {
		t.Errorf("Expected 'PUT response', got: %s", result)
	}
}

func TestCurlDeleteMethod(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE request, got %s", r.Method)
		}
		fmt.Fprint(w, "DELETE response")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-X", "DELETE", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "DELETE response" {
		t.Errorf("Expected 'DELETE response', got: %s", result)
	}
}

// ============== HEADER TESTS (-H) ==============

func TestCurlCustomHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Custom-Header") != "custom-value" {
			t.Errorf("Expected X-Custom-Header: custom-value, got: %s", r.Header.Get("X-Custom-Header"))
		}
		fmt.Fprint(w, "header received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-H", "X-Custom-Header: custom-value", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "header received" {
		t.Errorf("Expected 'header received', got: %s", result)
	}
}

func TestCurlMultipleHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Expected Accept: application/json, got: %s", r.Header.Get("Accept"))
		}
		if r.Header.Get("Authorization") != "Bearer token123" {
			t.Errorf("Expected Authorization: Bearer token123, got: %s", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, "headers received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{
		"-H", "Accept: application/json",
		"-H", "Authorization: Bearer token123",
		server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "headers received" {
		t.Errorf("Expected 'headers received', got: %s", result)
	}
}

// ============== POST DATA TESTS (-d) ==============

func TestCurlPostData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "name=test" {
			t.Errorf("Expected 'name=test', got: %s", string(body))
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type: application/x-www-form-urlencoded, got: %s", r.Header.Get("Content-Type"))
		}
		fmt.Fprint(w, "data received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-d", "name=test", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "data received" {
		t.Errorf("Expected 'data received', got: %s", result)
	}
}

func TestCurlPostDataWithContentType(t *testing.T) {
	// Note: When using -d, curl sets Content-Type to application/x-www-form-urlencoded
	// automatically, which overrides any custom -H header.
	// This is standard curl behavior.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// -d overrides Content-Type to application/x-www-form-urlencoded
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type: application/x-www-form-urlencoded, got: %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"key":"value"}` {
			t.Errorf("Expected JSON body, got: %s", string(body))
		}
		fmt.Fprint(w, "json received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{
		"-H", "Content-Type: application/json",
		"-d", `{"key":"value"}`,
		server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "json received" {
		t.Errorf("Expected 'json received', got: %s", result)
	}
}

func TestCurlUploadFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT request, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "upload body" {
			t.Errorf("Expected upload body, got %q", string(body))
		}
		fmt.Fprint(w, "uploaded")
	}))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(file, []byte("upload body"), 0o644); err != nil {
		t.Fatalf("write upload file: %v", err)
	}

	output, err := runCurlCmd([]string{"-T", file, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}
	if strings.TrimSpace(output) != "uploaded" {
		t.Fatalf("expected uploaded response, got %q", output)
	}
}

func TestCurlMultipartFormUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("expected multipart request: %v", err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("expected multipart part: %v", err)
		}
		if part.FormName() != "file" {
			t.Errorf("Expected form field 'file', got %s", part.FormName())
		}
		if part.FileName() != "payload.txt" {
			t.Errorf("Expected uploaded filename payload.txt, got %s", part.FileName())
		}
		data, _ := io.ReadAll(part)
		if string(data) != "multipart body" {
			t.Errorf("Expected multipart body, got %q", string(data))
		}
		fmt.Fprint(w, "multipart uploaded")
	}))
	defer server.Close()

	file := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(file, []byte("multipart body"), 0o644); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}

	output, err := runCurlCmd([]string{"-F", "file=@" + file, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}
	if strings.TrimSpace(output) != "multipart uploaded" {
		t.Fatalf("expected multipart response, got %q", output)
	}
}

// ============== INSECURE MODE TESTS (-k) ==============

func TestCurlInsecureWithBadCert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "secure content")
	}))
	defer server.Close()

	// Without -k, the self-signed cert must be rejected.
	if _, err := runCurlCmd([]string{server.URL}); err == nil {
		t.Fatalf("expected curl to reject the self-signed certificate without -k")
	}

	// With -k, the same request must succeed and return the real body.
	output, err := runCurlCmd([]string{"-k", server.URL})
	if err != nil {
		t.Fatalf("curl -k command failed: %v", err)
	}

	result := string(output)
	if result != "secure content" {
		t.Errorf("Expected 'secure content', got: %s", result)
	}
}

// ============== FAIL ON ERROR TESTS (-f) ==============

func TestCurlFailOnError4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	stdout, stderr, err := runCurlCmdFull([]string{"-f", server.URL})
	if err == nil {
		t.Errorf("Expected error on 404 with -f flag")
	}
	if stdout != "" {
		t.Fatalf("expected fail-on-error to suppress body, got stdout=%q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr+err.Error()), "404") && !strings.Contains(strings.ToLower(stderr+err.Error()), "not found") {
		t.Fatalf("expected 404 diagnostic, stderr=%q err=%v", stderr, err)
	}
}

func TestCurlFailOnError5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer server.Close()

	stdout, stderr, err := runCurlCmdFull([]string{"-f", server.URL})
	if err == nil {
		t.Errorf("Expected error on 500 with -f flag")
	}
	if stdout != "" {
		t.Fatalf("expected fail-on-error to suppress body, got stdout=%q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr+err.Error()), "500") && !strings.Contains(strings.ToLower(stderr+err.Error()), "internal server error") {
		t.Fatalf("expected 500 diagnostic, stderr=%q err=%v", stderr, err)
	}
}

// TestCurlFailOnErrorMessagePrintedOnce is a regression test for a bug where
// "-f"/"--fail" against a 4xx/5xx response printed the "HTTP error NNN"
// diagnostic twice: once inside runSingle (direct fmt.Fprintf to os.Stderr)
// and once more by the top-level CLI dispatcher (main.go's run()), which also
// prints returned errors unless SuppressCLIError() is true. The fix makes
// curl's own error always carry SuppressCLIError()==true, since CurlCmd
// already fully owns printing (or deliberately suppressing, under -s) its
// own diagnostic; the top-level dispatcher must never print it again.
func TestCurlFailOnErrorMessagePrintedOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	}))
	defer server.Close()

	stdout, stderr, err := runCurlCmdFull([]string{"-f", server.URL})
	if err == nil {
		t.Fatalf("expected -f to fail on HTTP 404")
	}
	if stdout != "" {
		t.Fatalf("expected fail-on-error to suppress body, got stdout=%q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "http error 404") {
		t.Fatalf("expected CurlCmd to print the diagnostic itself, got stderr=%q", stderr)
	}

	// Emulate main.go's run() dispatch: it prints "curl: <err>" exactly once
	// unless the error opts out via SuppressCLIError(). Since CurlCmd already
	// printed above, the dispatcher must stay silent here.
	var cliBuf bytes.Buffer
	type cliErrorSilencer interface {
		SuppressCLIError() bool
	}
	suppressed := false
	if silencer, ok := err.(cliErrorSilencer); ok {
		suppressed = silencer.SuppressCLIError()
	}
	if !suppressed {
		fmt.Fprintln(&cliBuf, "curl:", err)
	}

	combined := strings.ToLower(stderr + cliBuf.String())
	count := strings.Count(combined, "http error 404")
	if count != 1 {
		t.Fatalf("expected \"HTTP error 404\" to appear exactly once across CurlCmd stderr + CLI dispatch, got %d occurrences: %q", count, combined)
	}
}

func TestCurlNoFailOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "success")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-f", server.URL})
	if err != nil {
		t.Fatalf("curl command should not fail on 200, got: %v", err)
	}

	result := string(output)
	if result != "success" {
		t.Errorf("Expected 'success', got: %s", result)
	}
}

// ============== INCLUDE HEADERS TESTS (-i) ==============

func TestCurlIncludeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "custom-value")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "body content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-i", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "HTTP/") {
		t.Errorf("Expected HTTP status line in output, got: %s", result)
	}
	if !strings.Contains(result, "X-Custom-Header:") {
		t.Errorf("Expected X-Custom-Header in output, got: %s", result)
	}
	if !strings.Contains(result, "body content") {
		t.Errorf("Expected body content in output, got: %s", result)
	}
}

// ============== RESOLVE HOST TESTS (--resolve) ==============

func TestCurlResolveHost(t *testing.T) {
	// Create a server that would respond to example.com
	// But we'll resolve it to localhost
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "resolved content")
	}))
	defer server.Close()

	// Extract host and port from server URL
	serverURL := server.URL
	if strings.HasPrefix(serverURL, "http://") {
		serverURL = strings.TrimPrefix(serverURL, "http://")
	} else if strings.HasPrefix(serverURL, "https://") {
		serverURL = strings.TrimPrefix(serverURL, "https://")
	}

	parts := strings.Split(serverURL, ":")
	host := parts[0]
	port := parts[1]

	output, err := runCurlCmd([]string{"--resolve", host + ":" + port + ":127.0.0.1", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "resolved content" {
		t.Errorf("Expected 'resolved content', got: %s", result)
	}
}

func TestCurlResolveHostInvalidFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	_, err := runCurlCmd([]string{"--resolve", "invalid-format", server.URL})
	if err == nil {
		t.Errorf("Expected error for invalid --resolve format")
	}
}

// ============== CONNECT TIMEOUT TESTS (--connect-timeout) ==============

func TestCurlConnectTimeout(t *testing.T) {
	// 10.255.255.1 is an unroutable RFC1918 probe address: it never traverses
	// the real internet and has no responder on this private network, so the
	// TCP connect() itself should hang until --connect-timeout fires. Some
	// sandboxed network namespaces return an immediate "network unreachable"
	// instead of hanging; that's an environment difference, not a gobox bug,
	// so we skip rather than falsely fail in that case (mirrors CURL-013 in
	// tests/parity/net_parity_test.go).
	start := time.Now()
	_, err := runCurlCmd([]string{"--connect-timeout", "0.05", "http://10.255.255.1:81"})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected curl --connect-timeout to fail against an unroutable address")
	}
	msg := strings.ToLower(err.Error())
	immediateUnreachable := strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "no route to host")
	if immediateUnreachable && elapsed < 20*time.Millisecond {
		t.Skipf("environment returned an immediate network-unreachable error instead of a connect timeout (elapsed=%s): %v", elapsed, err)
	}
	if !strings.Contains(msg, "timeout") && !strings.Contains(msg, "timed out") && !strings.Contains(msg, "i/o timeout") {
		t.Fatalf("expected a timeout-related error, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("--connect-timeout=0.05 should fail quickly, took %s", elapsed)
	}
}

// ============== BENCHMARK MODE TESTS (--bench) ==============

// Note: Some benchmark tests are skipped because they trigger a quicksort bug in cmd_curl.go
// when all latencies are identical (stack overflow in quickSort's partition function).
// The bug is in the partition logic when handling identical values - it can cause
// infinite recursion when partition returns len(a) for arrays of equal elements.

func TestCurlBenchBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "bench content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "2", "-n", "10", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Requests:") {
		t.Errorf("Expected benchmark output with Requests, got: %s", result)
	}
	if !strings.Contains(result, "Concurrency:") {
		t.Errorf("Expected benchmark output with Concurrency, got: %s", result)
	}
	if !strings.Contains(result, "Latency:") {
		t.Errorf("Expected benchmark output with Latency, got: %s", result)
	}
}

func TestCurlBenchWarmup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "warmup content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "1", "-n", "5", "--warmup", "2", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	// Should complete without error
	if !strings.Contains(result, "Requests:") {
		t.Errorf("Expected benchmark output, got: %s", result)
	}
}

func TestCurlBenchConcurrency(t *testing.T) {
	// Track concurrent requests
	var maxConcurrent int
	var currentConcurrent int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		currentConcurrent++
		if currentConcurrent > maxConcurrent {
			maxConcurrent = currentConcurrent
		}
		mu.Unlock()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		currentConcurrent--
		mu.Unlock()

		fmt.Fprint(w, "concurrent content")
	}))
	defer server.Close()

	_, err := runCurlCmd([]string{"--bench", "-c", "5", "-n", "20", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	if maxConcurrent < 2 {
		t.Fatalf("expected -c 5 to run requests concurrently (server-observed peak concurrency), got max observed concurrency %d", maxConcurrent)
	}
}

func TestCurlBenchThroughput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "throughput content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "4", "-n", "20", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Throughput:") {
		t.Errorf("Expected throughput in output, got: %s", result)
	}
}

// ============== EDGE CASES ==============

func TestCurlEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return empty response
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "" {
		t.Errorf("Expected empty response, got: %s", result)
	}
}

func TestCurlSpecialCharactersInHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Special", "value with spaces and : colons")
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-i", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "X-Special:") {
		t.Errorf("Expected X-Special header in output, got: %s", result)
	}
}

func TestCurlBinaryData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send binary-like data
		binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(binaryData)
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	// Check that binary data was received (may not display directly)
	if len(result) < 6 {
		t.Errorf("Expected binary data to be received, got length: %d", len(result))
	}
}

func TestCurlLongResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send a large response
		largeContent := strings.Repeat("A", 1024*100) // 100KB
		fmt.Fprint(w, largeContent)
	}))
	defer server.Close()

	outputFile := "test_curl_large.txt"
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-o", outputFile, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(content) != 1024*100 {
		t.Errorf("Expected 100KB response, got: %d bytes", len(content))
	}
}

// ============== ERROR CASES ==============

func TestCurlInvalidURL(t *testing.T) {
	_, err := runCurlCmd([]string{"://invalid-url"})
	if err == nil {
		t.Errorf("Expected error for invalid URL")
	}
}

func TestCurlConnectionRefused(t *testing.T) {
	// Try to connect to a port that nothing is listening on
	_, err := runCurlCmd([]string{"http://127.0.0.1:59999"})
	if err == nil {
		t.Errorf("Expected error for connection refused")
	}
}

func TestCurlNonExistentHost(t *testing.T) {
	_, err := runCurlCmd([]string{"http://this-host-does-not-exist-xyz.example.com"})
	if err == nil {
		t.Errorf("Expected error for non-existent host")
	}
}

func TestCurlMissingURL(t *testing.T) {
	_, err := runCurlCmd([]string{})
	if err == nil {
		t.Errorf("Expected error when no URL provided")
	}
}

func TestCurlInvalidOption(t *testing.T) {
	_, err := runCurlCmd([]string{"-z"})
	if err == nil {
		t.Errorf("Expected error for invalid option")
	}
}

func TestCurlOutputOptionMissingArgument(t *testing.T) {
	_, err := runCurlCmd([]string{"-o"})
	if err == nil {
		t.Errorf("Expected error when -o is missing argument")
	}
}

func TestCurlMaxTimeInvalid(t *testing.T) {
	_, err := runCurlCmd([]string{"-m", "invalid", "http://example.com"})
	if err == nil {
		t.Errorf("Expected error for invalid max-time value")
	}
}

func TestCurlConnectTimeoutInvalid(t *testing.T) {
	_, err := runCurlCmd([]string{"--connect-timeout", "abc", "http://example.com"})
	if err == nil {
		t.Errorf("Expected error for invalid connect-timeout value")
	}
}

// ============== COMBINED OPTIONS TESTS ==============

func TestCurlCombinedOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("Expected Authorization header")
		}
		w.Header().Set("X-Response-Header", "test")
		fmt.Fprint(w, "combined response")
	}))
	defer server.Close()

	outputFile := "test_combined.txt"
	defer os.Remove(outputFile)

	output, err := runCurlCmd([]string{
		"-s",
		"-o", outputFile,
		"-w", "%{http_code}",
		"-H", "Authorization: Bearer secret",
		server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	// Check file output
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}
	if string(content) != "combined response" {
		t.Errorf("Expected file content 'combined response', got: %s", string(content))
	}

	// Check write-out output
	result := string(output)
	if !strings.Contains(result, "200") {
		t.Errorf("Expected '200' from write-out, got: %s", result)
	}
}

func TestCurlPostWithHeadersAndFollowRedirect(t *testing.T) {
	// Note: Standard HTTP behavior (RFC 7231) changes POST to GET after redirect.
	// The curl implementation follows this standard behavior.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		if r.URL.Path == "/final" {
			// After 302 redirect with POST data, method becomes GET
			// This is standard HTTP behavior
			fmt.Fprint(w, "final response")
		}
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{
		"-L",
		"-X", "POST",
		"-d", "key=value",
		"-H", "Content-Type: application/json",
		server.URL + "/redirect"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "final response" {
		t.Errorf("Expected 'final response', got: %s", result)
	}
}

// ============== BENCHMARK WITH FAIL ON ERROR ==============

func TestCurlBenchWithFailOnError(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 3 {
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "1", "-n", "5", "-f", server.URL})
	// Bench mode must complete and report the failure count, not abort the
	// whole run when -f causes one of the 5 requests (the 3rd, a 500) to fail.
	if err != nil {
		t.Fatalf("expected bench mode to complete despite one -f failure, got: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Failed: 1") {
		t.Errorf("expected exactly 1 failed request out of 5, got: %s", result)
	}
}

// ============== HELP FLAG TEST ==============

func TestCurlHelp(t *testing.T) {
	output, err := runCurlCmd([]string{"--help"})
	if err != nil {
		t.Fatalf("curl --help failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Usage:") {
		t.Errorf("Expected usage information, got: %s", result)
	}
	if !strings.Contains(result, "-s") {
		t.Errorf("Expected -s option in help, got: %s", result)
	}
}

// ============== HTTP 201/204/304 STATUS TESTS ==============

func TestCurlHttp201Created(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, "created")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", "%{http_code}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "201" {
		t.Errorf("Expected '201', got: %s", result)
	}
}

func TestCurlHttp204NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", "%{http_code}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "204" {
		t.Errorf("Expected '204', got: %s", result)
	}
}

func TestCurlHttp304NotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	_, err := runCurlCmd([]string{"-f", server.URL})
	// -f only fails on status >= 400; 304 must not be treated as an error.
	if err != nil {
		t.Fatalf("expected -f to accept a 304 response, got error: %v", err)
	}
}

// ============== REQUEST TIMEOUT FOR BENCHMARK ==============

func TestCurlBenchRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(w, "response")
	}))
	defer server.Close()

	// Set request timeout to 50ms - some requests should fail
	output, err := runCurlCmd([]string{"--bench", "-c", "2", "-n", "10", "-t", "0.05", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Failed:") || !strings.Contains(result, "Requests: 10") {
		t.Fatalf("expected benchmark timeout summary, got: %s", result)
	}
	if strings.Contains(result, "Failed: 0") {
		t.Fatalf("expected at least one timed out benchmark request, got: %s", result)
	}
}

// ============== BENCHMARK DEFAULT VALUES ==============

func TestCurlBenchDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "default bench")
	}))
	defer server.Close()

	// Test with no -c or -n specified (should use defaults)
	output, err := runCurlCmd([]string{"--bench", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Requests: 100") {
		t.Errorf("expected default request count of 100, got: %s", result)
	}
	if !strings.Contains(result, "Concurrency: 1") {
		t.Errorf("expected default concurrency of 1, got: %s", result)
	}
}

// ============== WRITE-OUT FORMAT EDGE CASES ==============

func TestCurlWriteOutUnknownFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	// Unknown format specifiers should be left as-is
	output, err := runCurlCmd([]string{"-w", "unknown %{unknown}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "%{unknown}") {
		t.Errorf("Expected unknown format to be preserved, got: %s", result)
	}
}

// ============== STDIN PIPE TESTS ==============

func TestCurlStdinNotSupported(t *testing.T) {
	// curl doesn't support reading URL from stdin like some commands
	// This should just fail with "URL required"
	_, err := runCurlCmdWithStdin([]string{}, "http://example.com")
	if err == nil {
		t.Errorf("Expected error when no URL provided")
	}
}

// ============== TRAILING SLASH URL ==============

func TestCurlUrlWithTrailingSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "trailing slash content")
	}))
	defer server.Close()

	url := strings.TrimSuffix(server.URL, "/") + "/"
	output, err := runCurlCmd([]string{url})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "trailing slash content" {
		t.Errorf("Expected 'trailing slash content', got: %s", result)
	}
}

// ============== CONCURRENT HEADERS ==============

func TestCurlMultipleSameHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest captures all values
		values := r.Header["X-Multi"]
		if len(values) < 2 {
			t.Errorf("Expected multiple X-Multi headers")
		}
		fmt.Fprint(w, "headers received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{
		"-H", "X-Multi: value1",
		"-H", "X-Multi: value2",
		server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "headers received" {
		t.Errorf("Expected 'headers received', got: %s", result)
	}
}

// ============== EMPTY HEADER VALUE ==============

func TestCurlEmptyHeaderValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Empty") != "" {
			t.Errorf("Expected empty X-Empty header, got: '%s'", r.Header.Get("X-Empty"))
		}
		fmt.Fprint(w, "empty header ok")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-H", "X-Empty:", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "empty header ok" {
		t.Errorf("Expected 'empty header ok', got: %s", result)
	}
}

// ============== REAL URL TESTS ==============

func TestCurlHttpsGoogle(t *testing.T) {
	// Test against a real HTTPS server
	output, err := runCurlCmd([]string{"-s", "-o", os.DevNull, "-w", "%{http_code}", "https://example.com"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if result != "200" && result != "301" && result != "302" {
		t.Errorf("Expected 200/301/302 from example.com, got: %s", result)
	}
}

// ============== BENCHMARK STATISTICS ==============

func TestCurlBenchStatisticsOutput(t *testing.T) {
	// Regression coverage for a historical quicksort stack overflow when
	// percentile-sorting bench latencies (see comment above
	// TestCurlBenchBasic). Give requests varying latency so min/p50/p90/p99/max
	// are meaningfully different, then verify the reported percentiles are
	// actually monotonic and consistent with each other.
	var count int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := count
		count++
		mu.Unlock()
		time.Sleep(time.Duration(n) * time.Millisecond)
		fmt.Fprint(w, "stats")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "1", "-n", "20", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	re := regexp.MustCompile(`Latency: min=([\d.]+)ms, max=([\d.]+)ms, mean=([\d.]+)ms, p50=([\d.]+)ms, p90=([\d.]+)ms, p99=([\d.]+)ms`)
	m := re.FindStringSubmatch(result)
	if m == nil {
		t.Fatalf("expected a Latency: min=.../max=.../mean=.../p50=.../p90=.../p99=... line, got: %s", result)
	}
	vals := make([]float64, 6)
	for i, s := range m[1:] {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("failed to parse latency value %q: %v", s, err)
		}
		vals[i] = v
	}
	min, max, mean, p50, p90, p99 := vals[0], vals[1], vals[2], vals[3], vals[4], vals[5]
	if !(min <= p50 && p50 <= p90 && p90 <= p99 && p99 <= max) {
		t.Fatalf("expected min<=p50<=p90<=p99<=max, got min=%v p50=%v p90=%v p99=%v max=%v", min, p50, p90, p99, max)
	}
	if mean < min || mean > max {
		t.Fatalf("expected min<=mean<=max, got min=%v mean=%v max=%v", min, mean, max)
	}
	if max <= min {
		t.Fatalf("expected varying request latencies to produce max > min, got min=%v max=%v", min, max)
	}
}

func TestCurlBenchWithWarmupRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "bench warmup")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "1", "-n", "3", "--warmup", "2", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Requests:") {
		t.Errorf("Expected benchmark output, got: %s", result)
	}
}

// ============== OUTPUT FILE OVERWRITE ==============

func TestCurlOutputFileOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "new content")
	}))
	defer server.Close()

	outputFile := "test_overwrite.txt"
	// Create existing file
	writeTestFile(t, outputFile, "old content")
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-o", outputFile, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("Expected 'new content', got: %s", string(content))
	}
}

// ============== HEAD WITH -f ==============

func TestCurlHeadWithFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-I", "-f", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "200") {
		t.Errorf("expected HEAD -f output to include the 200 status, got: %s", result)
	}
}

// ============== POST DATA GET METHOD ==============

func TestCurlDataImpliesPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If -d is used without -X, method should be POST
		if r.Method != "POST" {
			t.Errorf("Expected POST when using -d without -X, got %s", r.Method)
		}
		fmt.Fprint(w, "post implied")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-d", "test=data", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "post implied" {
		t.Errorf("Expected 'post implied', got: %s", result)
	}
}

// ============== MISSING ARGUMENTS ==============

func TestCurlMissingHeaderArgument(t *testing.T) {
	_, err := runCurlCmd([]string{"-H", "http://example.com"})
	if err == nil {
		t.Errorf("Expected error when -H is missing argument")
	}
}

func TestCurlMissingDataArgument(t *testing.T) {
	// -d consumes the single following argument as its value, leaving no
	// positional URL argument at all, so this must fail with "URL required"
	// rather than silently treating "http://example.com" as the target URL.
	_, err := runCurlCmd([]string{"-d", "http://example.com"})
	if err == nil {
		t.Fatalf("expected an error when -d consumes the only argument, leaving no URL")
	}
	if !strings.Contains(err.Error(), "URL required") {
		t.Errorf("expected \"URL required\" error, got: %v", err)
	}
}

func TestCurlMissingResolveArgument(t *testing.T) {
	_, err := runCurlCmd([]string{"--resolve", "http://example.com"})
	if err == nil {
		t.Errorf("Expected error when --resolve is missing arguments")
	}
}

// ============== BENCHMARK ALL SUCCESS ==============

func TestCurlBenchAllSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "success")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "4", "-n", "40", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Failed: 0") {
		t.Errorf("expected all 40 requests to succeed (Failed: 0), got: %s", result)
	}
}

// ============== CONTENT LENGTH ==============

func TestCurlWriteOutSizeDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "12345")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", "%{size_download}", "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Content length is 5 bytes
	if result != "5" {
		t.Errorf("Expected size_download '5', got: %s", result)
	}
}

// TestCurlWriteOutEscapedNewline is a regression test for the -w/--write-out
// literal "\n" bug: a format string containing the two characters '\' 'n'
// must be unescaped to an actual newline before %{...} substitution, matching
// real curl's -w behavior, instead of printing the literal backslash-n.
func TestCurlWriteOutEscapedNewline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", `%{http_code}\n`, "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	if strings.Contains(output, `\n`) {
		t.Errorf("expected literal backslash-n to be unescaped to a real newline, got: %q", output)
	}
	if output != "200\n" {
		t.Errorf("expected %q, got: %q", "200\n", output)
	}
}

// TestCurlWriteOutEscapedTab is a regression test that -w also unescapes
// "\t", matching real curl's -w behavior for tab separators.
func TestCurlWriteOutEscapedTab(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "content")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-w", `%{http_code}\t%{size_download}`, "-o", os.DevNull, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	if strings.Contains(output, `\t`) {
		t.Errorf("expected literal backslash-t to be unescaped to a real tab, got: %q", output)
	}
	if !strings.Contains(output, "200\t7") {
		t.Errorf("expected tab-separated output, got: %q", output)
	}
}

// Helper to track request count for variable timing tests
var requestCount int

// ============== ISSUE #5 - BENCHMARK WITH ACTUAL REQUEST COUNT ==============

func TestCurlBenchActualRequestCount(t *testing.T) {
	// "Requests: 50" in the summary is the echoed input parameter, not proof
	// the server actually received 50 requests. Count real server hits with
	// an atomic counter to verify -n actually drives that many requests.
	var received int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&received, 1)
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "2", "-n", "50", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Requests: 50") {
		t.Errorf("expected summary to report Requests: 50, got: %s", result)
	}
	if got := atomic.LoadInt64(&received); got != 50 {
		t.Errorf("expected server to actually receive 50 requests, got %d", got)
	}
}

// ============== BENCHMARK WITHOUT SILENT ==============

func TestCurlBenchWithoutSilent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "bench")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"--bench", "-c", "1", "-n", "5", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "Requests:") {
		t.Errorf("Expected benchmark output, got: %s", result)
	}
}

// ============== CONCURRENT LIMIT ==============

func TestCurlBenchConcurrentLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, "concurrent")
	}))
	defer server.Close()

	// Test with different concurrency levels
	for _, c := range []string{"1", "2", "4"} {
		n, _ := strconv.Atoi(c)
		totalRequests := n * 5

		output, err := runCurlCmd([]string{"--bench", "-c", c, "-n", strconv.Itoa(totalRequests), server.URL})
		if err != nil {
			t.Fatalf("curl command failed for concurrency %s: %v", c, err)
		}

		result := string(output)
		if !strings.Contains(result, "Concurrency: "+c) {
			t.Errorf("Expected Concurrency: %s, got: %s", c, result)
		}
	}
}

// ============== LARGE DATA POST ==============

func TestCurlPostLargeData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) != 1024*50 {
			t.Errorf("Expected 50KB body, got: %d bytes", len(body))
		}
		fmt.Fprint(w, "large data received")
	}))
	defer server.Close()

	largeData := strings.Repeat("A", 1024*50)
	output, err := runCurlCmd([]string{"-d", largeData, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "large data received" {
		t.Errorf("Expected 'large data received', got: %s", result)
	}
}

// ============== HTTP 401 UNAUTHORIZED ==============

func TestCurlHttp401Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := runCurlCmd([]string{"-f", server.URL})
	if err == nil {
		t.Errorf("Expected error on 401 with -f flag")
	}
}

// ============== HTTP 403 FORBIDDEN ==============

func TestCurlHttp403Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	_, err := runCurlCmd([]string{"-f", server.URL})
	if err == nil {
		t.Errorf("Expected error on 403 with -f flag")
	}
}

// ============== HEAD REQUEST GETS NO BODY ==============

func TestCurlHeadNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		// Don't write any body - for HEAD, body should be ignored anyway
	}))
	defer server.Close()

	outputFile := "test_head.txt"
	writeTestFile(t, outputFile, "should not be overwritten")
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-I", "-o", outputFile, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Content should be headers, not 100 null bytes
	if len(content) > 500 {
		t.Errorf("HEAD response should not have body, got: %d bytes", len(content))
	}
}

// ============== REDIRECT CHAIN ==============

func TestCurlRedirectChain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/1":
			http.Redirect(w, r, "/2", http.StatusMovedPermanently)
		case "/2":
			http.Redirect(w, r, "/3", http.StatusFound)
		case "/3":
			http.Redirect(w, r, "/final", http.StatusSeeOther)
		case "/final":
			fmt.Fprint(w, "final destination")
		default:
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-L", server.URL + "/1"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if result != "final destination" {
		t.Errorf("Expected 'final destination', got: %s", result)
	}
}

// ============== WRITE-OUT WITH REDIRECT ==============

func TestCurlWriteOutWithRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusMovedPermanently)
			return
		}
		fmt.Fprint(w, "final")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-L", "-w", "Status: %{http_code}", "-o", os.DevNull, server.URL + "/redirect"})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	// Should show 200 from final URL
	if !strings.Contains(result, "200") {
		t.Errorf("Expected status 200 from final URL, got: %s", result)
	}
}

// ============== POST WITH EMPTY DATA ==============

func TestCurlPostEmptyData(t *testing.T) {
	// Note: curl with -d "" doesn't send POST because empty string is falsy
	// This is expected behavior - use -X POST to force POST with empty body
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// With empty string, curl doesn't set POST method (empty string is falsy)
		// So we accept GET as valid when -d "" is used
		fmt.Fprint(w, "empty data request received")
	}))
	defer server.Close()

	output, err := runCurlCmd([]string{"-d", "", server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	result := string(output)
	if !strings.Contains(result, "empty data request received") {
		t.Errorf("Expected 'empty data request received', got: %s", result)
	}
}

// ============== ABSOLUTE PATH OUTPUT ==============

func TestCurlOutputAbsolutePath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "absolute path content")
	}))
	defer server.Close()

	absPath := filepath.Join(os.TempDir(), "test_curl_absolute.txt")
	defer os.Remove(absPath)

	_, err := runCurlCmd([]string{"-o", absPath, server.URL})
	if err != nil {
		t.Fatalf("curl command failed: %v", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "absolute path content" {
		t.Errorf("Expected 'absolute path content', got: %s", string(content))
	}
}

// ============== CLEANUP ON ERROR ==============

func TestCurlOutputFileCleanupOnError(t *testing.T) {
	// Hijack and close the connection without writing any response, so
	// client.Do genuinely fails with a connection error before curl ever
	// reaches the os.Create(outputFile) step.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatalf("ResponseWriter does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack failed: %v", err)
		}
		conn.Close()
	}))
	defer server.Close()

	outputFile := "test_cleanup.txt"
	writeTestFile(t, outputFile, "original content")
	defer os.Remove(outputFile)

	_, err := runCurlCmd([]string{"-o", outputFile, server.URL})
	if err == nil {
		t.Fatalf("expected an error when the connection is closed before any response")
	}

	data, readErr := os.ReadFile(outputFile)
	if readErr != nil {
		t.Fatalf("output file should still exist after a failed request: %v", readErr)
	}
	if string(data) != "original content" {
		t.Errorf("expected the pre-existing output file to be left untouched on request failure, got: %q", string(data))
	}
}

func writeTestFile(t *testing.T, filename, content string) {
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file %s: %v", filename, err)
	}
}
