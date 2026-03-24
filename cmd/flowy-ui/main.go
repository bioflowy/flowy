// Command flowy-ui serves the flowy web management UI.
// It embeds the React SPA from web/dist/ and reverse-proxies
// /api/* requests to a flowy-server instance.
package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/bioflowy/flowy/web"
)

func main() {
	apiAddr := flag.String("api-addr", "http://localhost:8124", "address of flowy-server")
	listenAddr := flag.String("addr", ":3000", "address to listen on")
	flag.Parse()

	// Sub-filesystem: strip "dist" prefix so paths resolve as "index.html".
	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("flowy-ui: embed sub: %v", err)
	}

	// Startup assertion: fail fast if the embed is misconfigured.
	if _, err := fs.Stat(distFS, "index.html"); err != nil {
		log.Fatalf("flowy-ui: embedded web/dist/index.html not found — run 'make build-ui' first")
	}

	apiBase, err := url.Parse(*apiAddr)
	if err != nil {
		log.Fatalf("flowy-ui: parse api-addr: %v", err)
	}

	mux := http.NewServeMux()

	// Proxy /api/ to flowy-server (strip /api prefix before forwarding).
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(apiBase)
			r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, "/api")
			if r.Out.URL.Path == "" {
				r.Out.URL.Path = "/"
			}
			r.Out.URL.RawQuery = r.In.URL.RawQuery
			r.SetXForwarded()
		},
	}
	mux.Handle("/api/", proxy)

	// SPA: serve static assets; fallback to index.html for React Router paths.
	mux.Handle("/", spaHandler(distFS))

	log.Printf("flowy-ui: listening on %s (api: %s)", *listenAddr, *apiAddr)
	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

// spaHandler returns index.html for any path not found in the embedded FS.
// Required for React Router client-side navigation on hard refresh.
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// Pre-flight check: try opening the file BEFORE calling file server.
		// This avoids the ResponseWriter interception problem (Pitfall 1).
		f, err := fsys.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// File not found: fall back to index.html for React Router paths.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
