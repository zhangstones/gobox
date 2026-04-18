package net

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// twCmd implements a tiny web server
func TwCmd(args []string) error {
	var (
		port     int    = 8080
		dir      string = "."
		reuse    bool   = false
		bench    bool   = false
		showHelp bool   = false
	)

	// Parse flags manually
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			showHelp = true
		case arg == "-r" || arg == "--reuse":
			reuse = true
		case arg == "--bench":
			bench = true
		case (arg == "-p" || arg == "--port") && i+1 < len(args):
			i++
			var portStr string
			if strings.HasPrefix(arg, "--port=") {
				portStr = arg[7:]
			} else {
				portStr = args[i]
			}
			fmt.Sscanf(portStr, "%d", &port)
		case strings.HasPrefix(arg, "--port="):
			fmt.Sscanf(arg[7:], "%d", &port)
		case (arg == "-d" || arg == "--dir") && i+1 < len(args):
			i++
			if strings.HasPrefix(arg, "--dir=") {
				dir = arg[6:]
			} else {
				dir = args[i]
			}
		case strings.HasPrefix(arg, "--dir="):
			dir = arg[6:]
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option: %s", arg)
			}
			goto doneFlags
		}
		i++
	}
doneFlags:

	if showHelp {
		printTwUsage(os.Stdout)
		return nil
	}

	addr := fmt.Sprintf(":%d", port)

	// Setup handlers
	if bench {
		http.HandleFunc("/ping", handlePing)
		http.HandleFunc("/upload", handleUpload)
	} else {
		http.HandleFunc("/", MakeStaticHandler(dir))
	}

	// Configure HTTP server
	server := &http.Server{
		Addr: addr,
	}

	if reuse {
		// Enable SO_REUSEADDR via listener
		ln, err := reuseListener(addr)
		if err != nil {
			return fmt.Errorf("failed to create listener: %w", err)
		}
		fmt.Fprintf(os.Stderr, "tw: starting benchmark server on %s (SO_REUSEADDR)\n", addr)
		return server.Serve(ln)
	}

	fmt.Fprintf(os.Stderr, "tw: starting server on %s, serving %s\n", addr, dir)
	return server.ListenAndServe()
}

func printTwUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: gobox tw [OPTIONS]")
	fmt.Fprintln(w, "Tiny web server for serving static files or benchmark endpoints.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -p, --port=PORT   Listen on port (default 8080)")
	fmt.Fprintln(w, "  -d, --dir=DIR     Directory to serve (default current directory)")
	fmt.Fprintln(w, "  -r, --reuse       Enable SO_REUSEADDR")
	fmt.Fprintln(w, "  --bench            Enable benchmark mode")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Benchmark mode endpoints (--bench):")
	fmt.Fprintln(w, "  GET  /ping         Returns 200 OK with \"pong\"")
	fmt.Fprintln(w, "  POST /ping         Echoes request body back")
	fmt.Fprintln(w, "  POST /upload       Accepts file upload, returns size and status")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gobox tw -p 8080 -d /var/www")
	fmt.Fprintln(w, "  gobox tw --port=9000 --dir=/tmp")
	fmt.Fprintln(w, "  gobox tw --bench -p 8080")
}

// reuseListener creates a listener with SO_REUSEADDR
func reuseListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	} else if r.Method == "POST" {
		// Echo the body back
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		io.Copy(w, r.Body)
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error reading body: %v", err)
		return
	}

	size := len(body)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "upload size: %d, status: ok", size)
}

func MakeStaticHandler(dir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Sanitize path to prevent directory traversal
		if r.URL.Path == "/" {
			// Serve index.html if exists
			indexPath := filepath.Join(dir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Directory listing for /\n")
			return
		}

		// Clean the path to prevent traversal
		path := filepath.Clean(filepath.Join(dir, r.URL.Path))

		// Verify path is within dir
		absDir, _ := filepath.Abs(dir)
		absPath, _ := filepath.Abs(path)
		if !strings.HasPrefix(absPath, absDir) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if path is a directory
		info, err := os.Stat(path)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		if info.IsDir() {
			// Try to serve index.html in the directory
			indexPath := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			// List directory contents
			entries, err := os.ReadDir(path)
			if err != nil {
				http.Error(w, "Cannot read directory", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "<html><body><h1>Directory: %s</h1><ul>\n", r.URL.Path)
			for _, entry := range entries {
				name := entry.Name()
				link := filepath.Join(r.URL.Path, name)
				if entry.IsDir() {
					link += "/"
					fmt.Fprintf(w, "<li><a href=\"%s\">%s/</a></li>\n", link, name)
				} else {
					fmt.Fprintf(w, "<li><a href=\"%s\">%s</a></li>\n", link, name)
				}
			}
			fmt.Fprintf(w, "</ul></body></html>\n")
			return
		}

		http.ServeFile(w, r, path)
	}
}
