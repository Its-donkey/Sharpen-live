//go:build !js && !wasm

package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:4173", "address to serve the static UI")
	staticDir := flag.String("dir", "ui", "directory containing the built UI files")
	apiTarget := flag.String("api", "http://127.0.0.1:8880", "base URL for the alert server API")
	flag.Parse()

	logger := newServerLogger()

	root, err := filepath.Abs(*staticDir)
	if err != nil {
		log.Fatalf("failed to resolve static directory: %v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		log.Fatalf("static directory %s is invalid: %v", root, err)
	}

	apiURL, err := url.Parse(*apiTarget)
	if err != nil || apiURL.Scheme == "" {
		log.Fatalf("invalid API target %q: %v", *apiTarget, err)
	}

	mime.AddExtensionType(".wasm", "application/wasm")

	mux := http.NewServeMux()
	mux.Handle("/api/", apiProxyHandler(apiURL))
	mux.Handle("/streamers/watch", localStreamersWatchHandler(filepath.Join(root, "streamers.json")))
	mux.Handle("/admin/logs", logsStreamHandler(apiURL))
	mux.Handle("/", staticHandler(root))

	logger.Printf("Serving alGUI from %s on http://%s (proxying /api to %s)", root, *listen, apiURL)
	if err := http.ListenAndServe(*listen, withHTTPLogging(mux, logger)); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func apiProxyHandler(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Host = target.Host
		proxy.ServeHTTP(w, r)
	})
}

func staticHandler(root string) http.Handler {
	fileServer := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		if strings.HasSuffix(r.URL.Path, ".wasm") {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fileServer.ServeHTTP(w, r)
	})
}

func localStreamersWatchHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		mod, err := fileModTime(path)
		if err != nil {
			http.Error(w, fmt.Sprintf("stat streamers: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "event: ready\ndata: %d\n\n", mod.UnixMilli())
		flusher.Flush()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		last := mod
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				current, err := fileModTime(path)
				if err != nil {
					continue
				}
				if current.After(last) {
					last = current
					fmt.Fprintf(w, "event: change\ndata: %d\n\n", current.UnixMilli())
					flusher.Flush()
				}
			}
		}
	})
}

func fileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
