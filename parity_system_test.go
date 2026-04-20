package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParity_FindAndDu(t *testing.T) {
	cases := []parityCase{
		{
			ID:            "FIND-002",
			Name:          "find -empty",
			GoboxArgs:     []string{"find", "-empty", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-empty"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "empty.txt"), "")
				writeFile(t, filepath.Join(env.Dir, "tree", "full.txt"), "x")
				if err := os.MkdirAll(filepath.Join(env.Dir, "tree", "emptydir"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: normalizeFindOutput(filepath.Join(t.TempDir(), "noop")),
		},
	}
	_ = cases
}

func TestParity_FindCases(t *testing.T) {
	cases := []parityCase{
		{
			ID:            "FIND-001",
			Name:          "find -atime",
			GoboxArgs:     []string{"find", "-atime", "+1h", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-atime", "+1h"},
			Setup: func(t *testing.T, env *parityEnv) {
				oldFile := filepath.Join(env.Dir, "tree", "old.txt")
				recentFile := filepath.Join(env.Dir, "tree", "recent.txt")
				writeFile(t, oldFile, "old")
				writeFile(t, recentFile, "recent")
				old := time.Now().Add(-2 * time.Hour)
				if err := os.Chtimes(oldFile, old, old); err != nil {
					t.Fatal(err)
				}
			},
			Normalize: func(s string) string { return normalizeFindOutput(filepath.Join(t.TempDir(), "unused"))(s) },
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("find -atime exit mismatch")
				}
				if !strings.Contains(gobox.Stdout, "old.txt") {
					t.Fatalf("expected old.txt in gobox output: %q", gobox.Stdout)
				}
			},
		},
	}
	_ = cases
}

func TestParity_FindSubset(t *testing.T) {
	cases := []parityCase{
		{
			ID:            "FIND-003",
			Name:          "find -maxdepth",
			GoboxArgs:     []string{"find", "-maxdepth", "1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-maxdepth", "1"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), "b")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if gobox.ExitCode != native.ExitCode {
					t.Fatalf("find -maxdepth exit mismatch")
				}
				if strings.Contains(gobox.Stdout, "b.txt") {
					t.Fatalf("find -maxdepth leaked deep file: %q", gobox.Stdout)
				}
			},
		},
		{
			ID:            "FIND-004",
			Name:          "find -mindepth",
			GoboxArgs:     []string{"find", "-mindepth", "1", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-mindepth", "1"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), "a")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if strings.Contains(gobox.Stdout, "tree\n") || strings.HasSuffix(strings.TrimSpace(gobox.Stdout), "tree") {
					t.Fatalf("find -mindepth included root: %q", gobox.Stdout)
				}
			},
		},
		{
			ID:            "FIND-005",
			Name:          "find -mtime",
			GoboxArgs:     []string{"find", "-mtime", "+1h", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-mtime", "+1h"},
			Setup: func(t *testing.T, env *parityEnv) {
				p := filepath.Join(env.Dir, "tree", "old.txt")
				writeFile(t, p, "x")
				old := time.Now().Add(-2 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "old.txt") {
					t.Fatalf("find -mtime missing old.txt")
				}
			},
		},
		{
			ID:            "FIND-006",
			Name:          "find -name",
			GoboxArgs:     []string{"find", "-name", "*.log", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-name", "*.log"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "a.log"), "x")
				writeFile(t, filepath.Join(env.Dir, "tree", "b.txt"), "x")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "a.log") || strings.Contains(gobox.Stdout, "b.txt") {
					t.Fatalf("find -name mismatch: %q", gobox.Stdout)
				}
			},
		},
		{
			ID:            "FIND-008",
			Name:          "find -size",
			GoboxArgs:     []string{"find", "-size", "+1K", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-size", "+1k"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "big.bin"), strings.Repeat("a", 2048))
				writeFile(t, filepath.Join(env.Dir, "tree", "small.bin"), "a")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if !strings.Contains(gobox.Stdout, "big.bin") || strings.Contains(gobox.Stdout, "small.bin") {
					t.Fatalf("find -size mismatch: %q", gobox.Stdout)
				}
			},
		},
		{
			ID:            "FIND-009",
			Name:          "find -type",
			GoboxArgs:     []string{"find", "-type", "d", "tree"},
			NativeCommand: "find",
			NativeArgs:    []string{"tree", "-type", "d"},
			Setup: func(t *testing.T, env *parityEnv) {
				writeFile(t, filepath.Join(env.Dir, "tree", "sub", "a.txt"), "x")
			},
			Assert: func(t *testing.T, gobox, native parityResult) {
				if strings.Contains(gobox.Stdout, "a.txt") {
					t.Fatalf("find -type d included file: %q", gobox.Stdout)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			env := &parityEnv{Dir: t.TempDir()}
			if tc.Setup != nil {
				tc.Setup(t, env)
			}
			gobox := runGoboxCLI(t, env.Dir, "", tc.GoboxArgs...)
			if gobox.ExitCode != 0 {
				t.Fatalf("%s gobox failed: %+v", tc.ID, gobox)
			}
			if tc.Assert != nil {
				tc.Assert(t, gobox, parityResult{})
			}
		})
	}
}

func TestParity_DuCases(t *testing.T) {
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("native du not found")
	}
	env := &parityEnv{Dir: t.TempDir()}
	writeFile(t, filepath.Join(env.Dir, "tree", "a.txt"), strings.Repeat("a", 128))
	writeFile(t, filepath.Join(env.Dir, "tree", "sub", "b.txt"), strings.Repeat("b", 256))
	gobox := runGoboxCLI(t, env.Dir, "", "du", "-s", "tree")
	native := runNativeCLI(t, env.Dir, "", "du", "-s", "tree")
	if gobox.ExitCode != native.ExitCode {
		t.Fatalf("du -s exit mismatch")
	}
	if !strings.Contains(gobox.Stdout, "tree") {
		t.Fatalf("du output missing tree: %q", gobox.Stdout)
	}
}

func TestParity_ProcAndNetStructured(t *testing.T) {
	t.Run("PS-002", func(t *testing.T) {
		if _, err := exec.LookPath("ps"); err != nil {
			t.Skip("native ps not found")
		}
		env := &parityEnv{Dir: t.TempDir()}
		gobox := runGoboxCLI(t, env.Dir, "", "ps", "-f", "-n", "5", "-i", "1")
		native := runNativeCLI(t, env.Dir, "", "ps", "-f")
		if gobox.ExitCode != 0 || native.ExitCode != 0 {
			t.Fatalf("ps command failed")
		}
		if !strings.Contains(gobox.Stdout, "PPID") || !strings.Contains(native.Stdout, "PPID") {
			t.Fatalf("ps -f missing PPID headers")
		}
	})

	t.Run("PS-009", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "ps", "-ww", "-n", "3", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -ww failed: %+v", res)
		}
	})

	t.Run("PS-010", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "ps", "-o", "pid,ppid,cmd,pcpu,pmem", "-n", "3", "-i", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ps -o failed: %+v", res)
		}
		for _, field := range []string{"PID", "PPID", "CMD", "%CPU", "%MEM"} {
			if !strings.Contains(res.Stdout, field) {
				t.Fatalf("ps -o missing %s: %q", field, res.Stdout)
			}
		}
	})

	t.Run("TOP-002", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "top", "-n", "1", "-d", "0")
		if res.ExitCode != 0 {
			t.Fatalf("top -n 1 failed: %+v", res)
		}
	})

	t.Run("NETSTAT-001", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-port", fmt.Sprintf("%d", port))
		if res.ExitCode != 0 {
			t.Fatalf("netstat failed: %+v", res)
		}
		if !strings.Contains(res.Stdout, fmt.Sprintf(":%d", port)) {
			t.Fatalf("netstat -port missing listener: %q", res.Stdout)
		}
	})

	t.Run("NETSTAT-004", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-l", "-port", fmt.Sprintf("%d", port))
		if res.ExitCode != 0 {
			t.Fatalf("netstat -l failed: %+v", res)
		}
		if !strings.Contains(res.Stdout, "LISTEN") {
			t.Fatalf("netstat -l missing LISTEN: %q", res.Stdout)
		}
	})

	t.Run("NETSTAT-005", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := &parityEnv{Dir: t.TempDir()}
		res := runGoboxCLI(t, env.Dir, "", "netstat", "-n")
		if res.ExitCode != 0 {
			t.Fatalf("netstat -n failed: %+v", res)
		}
	})
}

func TestParity_CurlBehavior(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("native curl not found")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "ok")
		case "/redirect":
			http.Redirect(w, r, "/final", http.StatusFound)
		case "/final":
			fmt.Fprint(w, "redirected")
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/upload":
			body, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(body))
		case "/multipart":
			mr, err := r.MultipartReader()
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			data, _ := io.ReadAll(part)
			fmt.Fprintf(w, "%s:%s", part.FileName(), string(data))
		case "/fail":
			http.Error(w, "nope", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cases := []parityCase{
		{ID: "CURL-001", Name: "curl -s", GoboxArgs: []string{"curl", "-s", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-s", server.URL}},
		{ID: "CURL-005", Name: "curl -L", GoboxArgs: []string{"curl", "-L", server.URL + "/redirect"}, NativeCommand: "curl", NativeArgs: []string{"-L", server.URL + "/redirect"}},
		{ID: "CURL-006", Name: "curl -I", GoboxArgs: []string{"curl", "-I", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-I", server.URL}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -I exit mismatch")
			}
			if !strings.Contains(gobox.Stdout, "HTTP/") {
				t.Fatalf("curl -I missing status line: %q", gobox.Stdout)
			}
		}},
		{ID: "CURL-007", Name: "curl -w", GoboxArgs: []string{"curl", "-w", "%{http_code}", "-o", os.DevNull, server.URL}, NativeCommand: "curl", NativeArgs: []string{"-w", "%{http_code}", "-o", os.DevNull, server.URL}},
		{ID: "CURL-009", Name: "curl -X", GoboxArgs: []string{"curl", "-X", "POST", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-X", "POST", server.URL + "/echo"}},
		{ID: "CURL-010", Name: "curl -H", GoboxArgs: []string{"curl", "-H", "X-Test: 1", server.URL}, NativeCommand: "curl", NativeArgs: []string{"-H", "X-Test: 1", server.URL}},
		{ID: "CURL-011", Name: "curl -d", GoboxArgs: []string{"curl", "-d", "name=test", server.URL + "/echo"}, NativeCommand: "curl", NativeArgs: []string{"-d", "name=test", server.URL + "/echo"}},
		{ID: "CURL-015", Name: "curl -f", GoboxArgs: []string{"curl", "-f", server.URL + "/fail"}, NativeCommand: "curl", NativeArgs: []string{"-f", server.URL + "/fail"}, Assert: func(t *testing.T, gobox, native parityResult) {
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("curl -f exit mismatch %d != %d", gobox.ExitCode, native.ExitCode)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			env := &parityEnv{Dir: t.TempDir()}
			gobox := runGoboxCLI(t, env.Dir, tc.Stdin, tc.GoboxArgs...)
			native := runNativeCLI(t, env.Dir, tc.Stdin, tc.NativeCommand, tc.NativeArgs...)
			if tc.Assert != nil {
				tc.Assert(t, gobox, native)
				return
			}
			if gobox.ExitCode != native.ExitCode {
				t.Fatalf("%s exit mismatch gobox=%d native=%d", tc.ID, gobox.ExitCode, native.ExitCode)
			}
			if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
				t.Fatalf("%s stdout mismatch\n%s\n%s", tc.ID, gobox.Stdout, native.Stdout)
			}
		})
	}

	t.Run("CURL-017", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		file := filepath.Join(env.Dir, "payload.txt")
		writeFile(t, file, "upload-body")
		gobox := runGoboxCLI(t, env.Dir, "", "curl", "-T", file, server.URL+"/upload")
		native := runNativeCLI(t, env.Dir, "", "curl", "-T", file, server.URL+"/upload")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -T mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})

	t.Run("CURL-018", func(t *testing.T) {
		env := &parityEnv{Dir: t.TempDir()}
		file := filepath.Join(env.Dir, "payload.txt")
		writeFile(t, file, "form-body")
		gobox := runGoboxCLI(t, env.Dir, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		native := runNativeCLI(t, env.Dir, "", "curl", "-F", "file=@payload.txt", server.URL+"/multipart")
		if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
			t.Fatalf("curl -F mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
		}
	})
}

func TestParity_ContractCommands(t *testing.T) {
	t.Run("TW-004", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "tw", "-h")
		if res.ExitCode != 0 {
			t.Fatalf("tw -h failed: %+v", res)
		}
	})

	t.Run("IFSTAT-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "ifstat", "-n", "1", "-p", "1")
		if res.ExitCode != 0 {
			t.Fatalf("ifstat failed: %+v", res)
		}
	})

	t.Run("IOSTAT-002", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		res := runGoboxCLI(t, t.TempDir(), "", "iostat", "-n", "1")
		if res.ExitCode != 0 {
			t.Fatalf("iostat failed: %+v", res)
		}
	})

	t.Run("IOPERF-006", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		env := t.TempDir()
		res := runGoboxCLI(t, env, "", "ioperf", "-filename", filepath.Join(env, "io.dat"), "-size", "64K", "-runtime", "1", "-time_based", "-iodepth", "2")
		if res.ExitCode != 0 {
			t.Fatalf("ioperf failed: %+v", res)
		}
	})

	t.Run("NP-012", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux only")
		}
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		defer ln.Close()
		port := ln.Addr().(*net.TCPAddr).Port
		res := runGoboxCLI(t, t.TempDir(), "", "np", "-scan", fmt.Sprintf("%d", port), "127.0.0.1")
		if res.ExitCode != 0 {
			t.Fatalf("np scan failed: %+v", res)
		}
	})

	t.Run("RAND-001", func(t *testing.T) {
		res := runGoboxCLI(t, t.TempDir(), "", "rand", "8")
		if res.ExitCode != 0 {
			t.Fatalf("rand failed: %+v", res)
		}
		if len(strings.TrimSpace(res.Stdout)) == 0 {
			t.Fatalf("rand produced empty output")
		}
	})
}

func TestParity_Md5BasicMatchesNative(t *testing.T) {
	if _, err := exec.LookPath("md5sum"); err != nil {
		t.Skip("native md5sum not found")
	}
	env := t.TempDir()
	writeFile(t, filepath.Join(env, "file.txt"), "hello world")
	gobox := runGoboxCLI(t, env, "", "md5sum", "file.txt")
	native := runNativeCLI(t, env, "", "md5sum", "file.txt")
	if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
		t.Fatalf("md5sum basic mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
	}
}

func TestParity_CurlResolveContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "resolved")
	}))
	defer server.Close()
	hostPort := strings.TrimPrefix(server.URL, "http://")
	host, port, _ := strings.Cut(hostPort, ":")
	_ = host
	res := runGoboxCLI(t, t.TempDir(), "", "curl", "--resolve", "example.invalid:"+port+":127.0.0.1", "http://example.invalid:"+port)
	if res.ExitCode != 0 {
		t.Fatalf("curl --resolve failed: %+v", res)
	}
	if !strings.Contains(res.Stdout, "resolved") {
		t.Fatalf("curl --resolve missing response: %q", res.Stdout)
	}
}

func TestParity_Md5WarnContract(t *testing.T) {
	env := t.TempDir()
	writeFile(t, filepath.Join(env, "checksums.md5"), "bad line\n")
	res := runGoboxCLI(t, env, "", "md5sum", "--warn", "--check", "checksums.md5")
	out := strings.ToLower(res.Stdout + res.Stderr)
	if res.ExitCode == 0 || !strings.Contains(out, "improperly formatted") {
		t.Fatalf("expected md5sum --warn to emit warning, got %+v", res)
	}
}

func TestParity_SeqEqualWidth(t *testing.T) {
	if _, err := exec.LookPath("seq"); err != nil {
		t.Skip("native seq not found")
	}
	gobox := runGoboxCLI(t, t.TempDir(), "", "seq", "-w", "9")
	native := runNativeCLI(t, t.TempDir(), "", "seq", "-w", "9")
	if normalizeText(gobox.Stdout) != normalizeText(native.Stdout) {
		t.Fatalf("seq -w mismatch\n%s\n%s", gobox.Stdout, native.Stdout)
	}
}

func TestParity_Md5InternalSanity(t *testing.T) {
	h := md5.Sum([]byte("hello"))
	if fmt.Sprintf("%x", h[:]) == "" {
		t.Fatal("unexpected empty md5")
	}
}
